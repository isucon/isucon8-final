require 'json'
require 'time'

require 'mysql2'
require 'mysql2-cs-bind'
require 'bcrypt'

require 'sinatra/base'

require 'isubank'
require 'isulogger'

module Isucoin
  class Web < Sinatra::Base
    class Error < StandardError; end
    class BankUserNotFound < Error; def initialize(*); super "bank user not found"; end; end
    class BankUserConflict < Error; def initialize(*); super "bank user conflict"; end; end
    class UserNotFound < Error; def initialize(*); super "user not found"; end; end
    class OrderNotFound < Error; def initialize(*); super "order not found"; end; end
    class OrderAlreadyClosed < Error; def initialize(*); super "order is already closed"; end; end
    class CreditInsufficient < Error; def initialize(*); super "銀行の残高が足りません"; end; end
    class ParameterInvalid < Error; def initialize(*); super "parameter invalid"; end; end
    class NoOrderForTrade < Error; def initialize(*); super "no order for trade"; end; end

    class NotAuthenticated < Error; def initialize(*); super "Not authenticated"; end; end

    configure :development do
      require 'sinatra/reloader'
      register Sinatra::Reloader
    end

    set :public_folder, ENV.fetch('ISU_PUBLIC_DIR', File.join(__dir__, '..', 'public'))
    set :sessions, key: 'isucoin_session', expire_after: 3600
    set :session_secret, 'tonymoris'

    helpers do
      def db
        Thread.current[:db] ||= Mysql2::Client.new(
          host: ENV.fetch('ISU_DB_HOST', '127.0.0.1'),
          port: ENV.fetch('ISU_DB_PORT', '3306'),
          username: ENV.fetch('ISU_DB_USER', 'root'),
          password: ENV['ISU_DB_PASSWORD'],
          database: ENV.fetch('ISU_DB_NAME', 'isucoin'),
          cast_booleans: true,
          reconnect: true,
        )
      end

      def isubank
        Isubank.new(get_setting('bank_endpoint'), get_setting('bank_appid'))
      end

      def isulogger
        Isulogger.new(get_setting('log_endpoint'), get_setting('log_appid'))
      end

      def get_setting(k)
        db.xquery('SELECT val FROM setting WHERE name = ?', k).first.fetch('val')
      end

      def set_setting(k, v)
        db.xquery('INSERT INTO setting (name, val) VALUES (?, ?) ON DUPLICATE KEY UPDATE val = VALUES(val)', k, v)
      end

      def send_log(tag, v)
        isulogger.send(tag, v)
      end

      def user_signup(name, bank_id, password)
        # bank_id の検証
        isubank.check(bank_id, 0)
        pass = BCrypt::Password.create(password)
        db.xquery('INSERT INTO user (bank_id, name, password, created_at) VALUES (?, ?, ?, NOW(6))', bank_id, name, pass)
        user_id = db.last_id
        isulogger.send('signup', bank_id: bank_id, user_id: user_id, name: name)
      rescue Isubank::NoUserError
        raise BankUserNotFound
      rescue Mysql2::Error => e
        if e.error_number == 1062
          raise BankUserConflict 
        end
        raise e
      end

      def user_login(bank_id, password)
        user = db.xquery('SELECT * FROM user WHERE bank_id = ?', bank_id).first
        raise UserNotFound unless user
        raise UserNotFound unless BCrypt::Password.new(user.fetch('password')) == password
        send_log('signin', bank_id: user.fetch('bank_id'), user_id: user.fetch('id'), name: user.fetch('name'))

        user
      end

      def get_user_by_id(id)
        db.xquery('SELECT * FROM user WHERE id = ?', id).first
      end

      def get_user_by_id_with_lock(id)
        db.xquery('SELECT * FROM user WHERE id = ? FOR UPDATE', id).first
      end

      def user_by_request
        if session[:user_id]
          user = get_user_by_id(session[:user_id])
          if user
            return user
          end
        end
      end

      def get_orders_by_user_id(user_id)
        db.xquery('SELECT * FROM orders WHERE user_id = ? AND (closed_at IS NULL OR trade_id IS NOT NULL) ORDER BY created_at ASC', user_id)
      end

      def get_orders_by_user_id_and_last_trade_id(user_id, trade_id)
        db.xquery('SELECT * FROM orders WHERE user_id = ? AND trade_id IS NOT NULL AND trade_id > ? ORDER BY created_at ASC', user_id, trade_id)
      end

      def get_open_order_by_id(id)
        order = get_order_by_id_with_lock(id)
        raise Error.new("no order with id=#{id}") unless order

        if order.fetch('closed_at')
          raise OrderAlreadyClosed
        end
        order[:user] = get_user_by_id_with_lock(order.fetch('user_id'))
        order
      end

      def get_order_by_id(id)
        db.xquery('SELECT * FROM orders WHERE id = ?', id).first
      end

      def get_order_by_id_with_lock(id)
        db.xquery('SELECT * FROM orders WHERE id = ? FOR UPDATE', id).first
      end

      def get_lowest_sell_order
        db.xquery('SELECT * FROM orders WHERE type = ? AND closed_at IS NULL ORDER BY price ASC, created_at ASC LIMIT 1', 'sell').first
      end

      def get_highest_buy_order
        db.xquery('SELECT * FROM orders WHERE type = ? AND closed_at IS NULL ORDER BY price DESC, created_at ASC LIMIT 1', 'buy').first
      end

      def fetch_order_relation(order)
        order[:user] = get_user_by_id(order.fetch('user_id'))
        if order.fetch('trade_id')
          order[:trade] = get_trade_by_id(order['trade_id'])
        end
        nil
      end

      def add_order(ot, user_id, amount, price)
        if amount <= 0 || price <= 0
          raise ParameterInvalid
        end

        user = get_user_by_id_with_lock(user_id)
        raise Error.new("no user with id=#{user_id}") unless user 

        case ot
        when 'buy'
          total_price = price * amount
          begin
            isubank.check(user.fetch('bank_id'), total_price)
          rescue Isubank::Error => e
            send_log('buy.error',
              error: e.message,
              user_id: user.fetch('id'),
              amount: amount,
              price: price,
            )
            if e.is_a?(Isubank::CreditInsufficientError)
              raise CreditInsufficient
            else
              raise e
            end
          end
        when 'sell'
          # TODO: 椅子の保有チェック
        else
          raise ParameterInvalid
        end

        db.xquery('INSERT INTO orders (type, user_id, amount, price, created_at) VALUES (?, ?, ?, ?, NOW(6))', ot, user.fetch('id'), amount, price)
        id = db.last_id()

        send_log("#{ot}.order",
          order_id: id,
          user_id: user['id'],
          amount: amount,
          price: price,
        )
        return get_order_by_id(id)
      end

      def delete_order(user_id, order_id, reason)
        user = get_user_by_id_with_lock(user_id)
        raise Error.new("no user with id=#{user_id}") unless user 

        order = get_order_by_id_with_lock(order_id)
        case 
        when !order
          raise OrderNotFound
        when order.fetch('user_id') != user.fetch('id')
          raise OrderNotFound
        when order.fetch('closed_at')
          raise OrderAlreadyClosed
        end

        cancel_order(order, reason)
      end

      def cancel_order(order, reason)
        db.xquery('UPDATE orders SET closed_at = NOW(6) WHERE id = ?', order.fetch('id'))
        send_log("#{order.fetch('type')}.delete",
          order_id: order['id'],
          user_id: order.fetch('user_id'),
          reason: reason,
        )
      end

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

    set :login_required, ->(_value) do
      condition do
        unless session[:user_id]
          halt 401, {code: 401, err: 'Not authenticated'}.to_json
        end
      end
    end

    before do
      content_type :json

      if session[:user_id]
        unless get_user_by_id(session[:user_id])
          session[:user_id] = nil
          halt 404, {code: 404, err: 'セッションが切断されました'}.to_json
        end
      end
    end

    get '/' do
      content_type :html
      File.read(File.join(__dir__, '..', 'public', 'index.html'))
    end

    post '/initialize' do
      # 前回の10:00:00+0900までのデータを消す
      # 本戦当日は2018-10-20T10:00:00+0900 固定だが、他の時間帯にデータ量を揃える必要がある
      stop = Time.now - (10 * 3600)
      stop = Time.local(stop.year, stop.month, stop.day, 10, 0, 0)

      [
        "DELETE FROM orders WHERE created_at >= ?",
        "DELETE FROM trade WHERE created_at >= ?",
        "DELETE FROM user WHERE created_at >= ?",
      ].each do |q|
        db.xquery(q, stop)
      end

      %i(
        bank_endpoint
        bank_appid
        log_endpoint
        log_appid
      ).each do |k|
        set_setting(k, params[k])
      end

      '{}'
    end

    post '/signup' do
      unless params[:name] && params[:bank_id] && params[:password]
        halt 400, {code: 400, err: "all parameters are required"}.to_json
      end

      begin
        user_signup(params[:name], params[:bank_id], params[:password])
      rescue BankUserNotFound => e
        halt 404, {code: 404, err: e.message}.to_json
      rescue BankUserConflict => e
        halt 409, {code: 409, err: e.message}.to_json
      end

      '{}'
    end

    post '/signin' do
      unless params[:bank_id] && params[:password]
        halt 400, {code: 400, err: "all parameters are required"}.to_json
      end

      begin
        user = user_login(params[:bank_id], params[:password])
        session[:user_id] = user.fetch('id')
      rescue UserNotFound => e
        # TODO: 失敗が多いときに403を返すBanの仕様に対応
        halt 404, {code: 404, err: e.message}.to_json
      end

      {
        id: user.fetch('id'),
        name: user.fetch('name'),
      }.to_json
    end

    post '/signout' do
      session[:user_id] = nil
      '{}'
    end

    get '/info' do
      res = {}

      last_trade_id = params[:cursor] && !params[:cursor].empty? ? params[:cursor].to_i : nil
      last_trade = last_trade_id && last_trade_id > 0 ? get_trade_by_id(last_trade_id) : nil
      lt = last_trade ? last_trade.fetch('created_at') : Time.at(0)

      latest_trade = get_latest_trade()
      res[:cursor] = latest_trade&.fetch('id')

      user = user_by_request()
      if user
        orders = get_orders_by_user_id_and_last_trade_id(user.fetch('id'), last_trade_id).to_a
        orders.each do |order|
          fetch_order_relation(order)
          order[:user] = {id: order.dig(:user, 'id'), name: order.dig(:user, 'name')} if order[:user]
          order[:trade]['created_at'] = order.dig(:trade, 'created_at').xmlschema if order.dig(:trade, 'created_at')
          order['created_at'] = order['created_at'].xmlschema if order['created_at']
          order['closed_at'] = order['closed_at'].xmlschema if order['closed_at']
        end
        res[:traded_orders] = orders
      end

      by_sec_time = Time.now - 300
      if by_sec_time < lt
        by_sec_time = Time.new(lt.year, lt.month, lt.day, lt.hour, lt.min, lt.sec)
      end
      res[:chart_by_sec] = get_candlestick_data(by_sec_time, "%Y-%m-%d %H:%i:%s")

      by_min_time = Time.now - (300 * 60)
      if by_min_time < lt
        by_min_time = Time.new(lt.year, lt.month, lt.day, lt.hour, lt.min, 0)
      end
      res[:chart_by_min] = get_candlestick_data(by_min_time, "%Y-%m-%d %H:%i:00")

      by_hour_time = Time.now - (48 * 3600)
      if by_hour_time < lt
        by_hour_time = Time.new(lt.year, lt.month, lt.day, lt.hour, 0, 0)
      end
      res[:chart_by_hour] = get_candlestick_data(by_hour_time, "%Y-%m-%d %H:00:00")

      lowest_sell_order = get_lowest_sell_order()
      res[:lowest_sell_price] = lowest_sell_order.fetch('price') if lowest_sell_order
      highest_buy_order = get_highest_buy_order()
      res[:highest_buy_price] = highest_buy_order.fetch('price') if highest_buy_order

      # TODO: trueにするとシェアボタンが有効になるが、アクセスが増えてヤバイので一旦falseにしておく
      res[:enable_share] = false

      %i(chart_by_hour chart_by_min chart_by_sec).each do |k|
        res[k].each do |cs|
          cs['time'] = cs.fetch('time').xmlschema
        end
      end
      res.to_json
    end

    post '/orders', login_required: true do
      user = user_by_request()
      amount = params[:amount]&.to_i
      price = params[:price]&.to_i

      begin
        rollback = false
        db.query('BEGIN')
        order = add_order(params[:type], user.fetch('id'), amount, price)
        db.query('COMMIT')
      rescue ParameterInvalid, CreditInsufficient => e
        rollback = true
        halt 400, {code: 400, err: e.message}.to_json
      ensure
        db.query('ROLLBACK') if $! || rollback
      end

      if has_trade_chance_by_order(order.fetch('id'))
        begin
          run_trade()
        rescue => e
          # トレードに失敗してもエラーにはしない
          $stderr.puts "run_trade error: #{e.full_message}"
        end
      end

      {id: order['id']}.to_json
    end

    get '/orders', login_required: true do
      user = user_by_request()
      orders = get_orders_by_user_id(user.fetch('id')).to_a

      orders.each do |order|
        fetch_order_relation(order)
        order[:user] = {id: order.dig(:user, 'id'), name: order.dig(:user, 'name')} if order[:user]
        order[:trade]['created_at'] = order.dig(:trade, 'created_at').xmlschema if order.dig(:trade, 'created_at')
        order['created_at'] = order['created_at'].xmlschema if order['created_at']
        order['closed_at'] = order['closed_at'].xmlschema if order['closed_at']
      end

      orders.to_json
    end

    delete '/order/:id', login_required: true do
      user = user_by_request()
      id = params[:id]&.to_i

      begin
        rollback = false
        db.query('BEGIN')
        delete_order(user.fetch('id'), id, 'canceled')
        db.query('COMMIT')
      rescue OrderNotFound, OrderAlreadyClosed => e
        rollback = true
        halt 404, {code: 404, err: e.message}.to_json
      ensure
        db.query('ROLLBACK') if $! || rollback
      end

      {id: id}.to_json
    end
  end
end
