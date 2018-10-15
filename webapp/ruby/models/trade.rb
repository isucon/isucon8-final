module Isucoin
  module Models
    module Trade
      def get_trade_by_id(id)
        db.xquery('SELECT * FROM trade WHERE id = ?', id).first
      end

      def get_latest_trade()
        db.xquery('SELECT * FROM trade ORDER BY id DESC').first
      end

      def get_candlestick_data(mt, tf)
        db.xquery(<<-EOF, mt).to_a
          SELECT m.t AS time, a.price AS open, b.price AS close, m.h AS high, m.l AS low
          FROM (
            SELECT
              STR_TO_DATE(DATE_FORMAT(created_at, '#{tf}'), '%Y-%m-%d %H:%i:%s') AS t,
              MIN(id) AS min_id,
              MAX(id) AS max_id,
              MAX(price) AS h,
              MIN(price) AS l
            FROM trade
            WHERE created_at >= ?
            GROUP BY t
          ) m
          JOIN trade a ON a.id = m.min_id
          JOIN trade b ON b.id = m.max_id
          ORDER BY m.t
        EOF
      end

      def has_trade_chance_by_order(order_id)
        order = get_order_by_id(order_id)
        raise Error.new("no order with id=#{order_id}", order_id) unless order

        lowest = get_lowest_sell_order()
        return false unless lowest

        highest = get_highest_buy_order()
        return false unless highest

        case order.fetch('type')
        when 'buy'
          return lowest.fetch('price') <= order.fetch('price')
        when 'sell'
          return order.fetch('price') <= highest.fetch('price')
        else
          raise Error.new("other type [#{order['type']}]")
        end

        false
      end

      def reserve_order(order, price)
        bank = isubank()
        total_price = order.fetch('amount') * price
        if order.fetch('type') == 'buy'
          total_price *= -1
        end

        return bank.reserve(order[:user].fetch('bank_id'), total_price)
      rescue Isubank::Error => e
        if e.is_a?(Isubank::CreditInsufficientError)
          cancel_order(order, "reserve_failed")
          send_log("#{order.fetch('type')}.error", 
            error: e.message,
            user_id: order.fetch('user_id'),
            amount: order.fetch('amount'),
            price: price,
          )
        end

        raise e
      end

      def commit_reserved_order(order, targets, reserves)
        db.xquery('INSERT INTO trade (amount, price, created_at) VALUES (?, ?, NOW(6))', order.fetch('amount'), order.fetch('price'))
        trade_id = db.last_id

        send_log("trade",
          trade_id: trade_id,
          price: order['price'],
          amount: order['amount'],
        )

        [*targets, order].each do |o|
          db.xquery('UPDATE orders SET trade_id = ?, closed_at = NOW(6) WHERE id = ?', trade_id, o.fetch('id'))
          send_log("#{o.fetch('type')}.trade",
            order_id: o['id'],
            price: order['price'],
            amount: o.fetch('amount'),
            user_id: o.fetch('user_id'),
            trade_id: trade_id,
          )
        end

        isubank.commit(reserves)
      end

      def try_trade(order_id)
        order = get_open_order_by_id(order_id)
        rest_amount = order.fetch('amount')
        unit_price = order.fetch('price')
        reserves = []
        targets = []

        reserves << reserve_order(order, unit_price)

        target_orders = case order.fetch('type')
        when 'buy'
          db.xquery('SELECT * FROM orders WHERE type = ? AND closed_at IS NULL AND price <= ? ORDER BY price ASC, created_at ASC, id ASC', 'sell', order.fetch('price')).to_a
        when 'sell'
          db.xquery('SELECT * FROM orders WHERE type = ? AND closed_at IS NULL AND price >= ? ORDER BY price DESC, created_at ASC, id ASC', 'buy', order.fetch('price')).to_a
        end
        raise NoOrderForTrade if target_orders.empty?

        target_orders.each do |to|
          begin
            to = get_open_order_by_id(to.fetch('id'))
          rescue OrderAlreadyClosed
            next
          end

          if to.fetch('amount') > rest_amount
            next
          end

          begin
            rid = reserve_order(to, unit_price)
          rescue Isubank::CreditInsufficientError
            next
          end

          reserves << rid
          targets << to

          rest_amount -= to.fetch('amount')
          break if rest_amount == 0
        end

        raise NoOrderForTrade if rest_amount > 0
        commit_reserved_order(order, targets, reserves)
        reserves = nil
      ensure
        isubank.cancel(reserves) if reserves && !reserves.empty?
      end

      def run_trade()
        lowest_sell_order = get_lowest_sell_order()
        # 売り注文が無いため成立しない
        return unless lowest_sell_order
        highest_buy_order = get_highest_buy_order()
        # 買い注文が無いため成立しない
        return unless highest_buy_order

        # 最安の売値が最高の買値よりも高いため成立しない
        if lowest_sell_order.fetch('price') > highest_buy_order.fetch('price')
          return nil
        end

        candidates = if lowest_sell_order.fetch('amount') > highest_buy_order.fetch('amount')
          [lowest_sell_order.fetch('id'), highest_buy_order.fetch('id')]
        else
          [highest_buy_order.fetch('id'), lowest_sell_order.fetch('id')]
        end

        candidates.each do |order_id|
          begin
            commit = false
            db.query('BEGIN')

            try_trade(order_id)
          rescue NoOrderForTrade, OrderAlreadyClosed
            # 注文個数の多い方で成立しなかったので少ない方で試す
            next
          rescue Isubank::CreditInsufficientError
            commit = true
            raise
          ensure
            if $! && !commit
              db.query('ROLLBACK')
            else
              db.query('COMMIT')
            end
          end

          return run_trade() # トレード成立したため次の取引を行う
        end

        # 個数のが不足していて不成立
        return nil
      end
    end
  end
end
