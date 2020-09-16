import db, { dbQuery } from '../db';
import {
    cancelOrder,
    CreditInsufficient,
    getHighestBuyOrder,
    getLowestSellOrder,
    getOpenOrderById,
    getOrderById,
    Order,
    OrderAlreadyClosed,
} from './orders';
import { getIsubank, sendLog } from './settings';
import { promisify } from 'util';

class NoOrderForTrade extends Error {
    constructor() {
        super('no order for trade');
        Object.setPrototypeOf(this, NoOrderForTrade.prototype);
    }
}

export class Trade {
    constructor(
        public id: number,
        public amount: number,
        public price: number,
        public created_at: Date
    ) {}
}

class CandlestickData {
    constructor(
        public time: Date,
        public open: number,
        public close: number,
        public high: number,
        public lower: number
    ) {}
}

async function getTrade(query: string, ...args: any[]): Promise<Trade | null> {
    const [row] = await dbQuery(query, args);
    if (!row) return null;
    return new Trade(row.id, row.amount, row.price, row.created_at);
}

export async function getTradeById(id: number) {
    return getTrade('SELECT * FROM trade WHERE id = ?', id);
}

export async function getLatestTrade() {
    return getTrade('SELECT * FROM trade ORDER BY id DESC');
}

export async function getCandlesticData(mt: Date, tf: string) {
    const query = `
        SELECT m.t, a.price as open, b.price as close, m.h, m.l
        FROM (
            SELECT
                STR_TO_DATE(DATE_FORMAT(created_at, ?), ?) AS t,
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
    `;
    const result = await dbQuery(query, [tf, '%Y-%m-%d %H:%i:%s', mt]);
    return result.map(
        (row: any) =>
            new CandlestickData(row.t, row.open, row.close, row.h, row.l)
    );
}

export async function hasTradeChanceByOrder(orderId: number) {
    const order = await getOrderById(orderId);
    const lowest = await getLowestSellOrder();
    if (!lowest) {
        return false;
    }
    const highest = await getHighestBuyOrder();
    if (!highest) {
        return false;
    }

    if (order?.type === 'buy' && lowest.price <= order.price) {
        return true;
    }
    if (order?.type === 'sell' && order.price <= highest.price) {
        return true;
    }

    return false;
}

async function reserveOrder(order: Order, price: number): Promise<number> {
    const bank = await getIsubank();
    let p = order.amount * price;
    if (order.type === 'buy') {
        p = -p;
    }
    try {
        return bank.reserve(order.user!.bank_id, p);
    } catch (e) {
        await cancelOrder(order, 'reserve_failed');
        await sendLog(order.type + '.error', {
            error: e.message,
            user_id: order.user_id,
            amount: order.amount,
            price: price,
        });
        throw e;
    }
}

async function commitReservedOrder(
    order: Order,
    targets: Order[],
    reserveIds: number[]
) {
    const {
        insertId,
    } = await dbQuery(
        'INSERT INTO trade (amount, price, created_at) VALUES (?, ?, NOW(6))',
        [order.amount, order.price]
    );

    const tradeId = insertId;
    sendLog('trade', {
        trade_id: tradeId,
        price: order.price,
        amount: order.amount,
    });

    for (const o of targets.concat([order])) {
        await dbQuery(
            'UPDATE orders SET trade_id = ?, closed_at = NOW(6) WHERE id = ?',
            [tradeId, o.id]
        );
        sendLog(o.type + '.trade', {
            order_id: o.id,
            price: order.price,
            amount: o.amount,
            user_id: o.user_id,
            trade_id: tradeId,
        });
    }

    const bank = await getIsubank();
    await bank.commit(reserveIds);
}

async function tryTrade(orderId: number) {
    const order = await getOpenOrderById(orderId);
    if (!order) {
        throw new Error('try trade error');
    }
    let restAmount = order.amount;
    const unitPrice = order.price;
    let reserves = [await reserveOrder(order, unitPrice)];

    try {
        let result: any[][];
        if (order.type === 'buy') {
            result = await dbQuery(
                'SELECT * FROM orders WHERE type = ? AND closed_at IS NULL AND price <= ? ORDER BY price ASC, created_at ASC, id ASC',
                ['sell', order.price]
            );
        } else {
            result = await dbQuery(
                'SELECT * FROM orders WHERE type = ? AND closed_at IS NULL AND price >= ? ORDER BY price DESC, created_at DESC, id DESC',
                ['buy', order.price]
            );
        }
        const targetOrders = result.map(
            (row: any) =>
                new Order(
                    row.id,
                    row.type,
                    row.user_id,
                    row.amount,
                    row.price,
                    row.closed_at,
                    row.trade_id,
                    row.created_at
                )
        );
        const targets: Order[] = [];
        for (let to of targetOrders) {
            try {
                to = (await getOpenOrderById(to.id)) as Order;
                if (!to) continue;
            } catch (e) {
                continue;
            }
            if (to.amount > restAmount) {
                continue;
            }
            try {
                const rid = await reserveOrder(to, unitPrice);
                reserves.push(rid);
            } catch (e) {
                continue;
            }
            targets.push(to);
            restAmount -= to.amount;
            if (restAmount === 0) {
                break;
            }
        }
        if (restAmount > 0) {
            throw new NoOrderForTrade();
        }
        await commitReservedOrder(order, targets, reserves);
        reserves = [];
    } finally {
        if (reserves.length) {
            const bank = await getIsubank();
            await bank.cancel(reserves);
        }
    }
}

export async function runTrade() {
    const lowestSellOrder = await getLowestSellOrder();
    if (!lowestSellOrder) {
        // 売り注文が無いため成立しない
        return;
    }
    const highestBuyOrder = await getHighestBuyOrder();
    if (!highestBuyOrder) {
        // 買い注文が無いため成立しない
        return;
    }
    if (lowestSellOrder.price > highestBuyOrder.price) {
        // 最安の売値が最高の買値よりも高いため成立しない
        return;
    }

    let candidates: number[];
    if (lowestSellOrder.amount > highestBuyOrder.amount) {
        candidates = [lowestSellOrder.id, highestBuyOrder.id];
    } else {
        candidates = [highestBuyOrder.id, lowestSellOrder.id];
    }

    for (const orderId of candidates) {
        await promisify(db.beginTransaction.bind(db))();
        try {
            await tryTrade(orderId);
            // トレード成立したため次の取引を行う
            await promisify(db.commit.bind(db))();
            await runTrade();
        } catch (e) {
            if (
                e instanceof NoOrderForTrade ||
                e instanceof OrderAlreadyClosed
            ) {
                // 注文個数の多い方で成立しなかったので少ない方で試す
                await promisify(db.commit.bind(db))();
                continue;
            } else if (e instanceof CreditInsufficient) {
                await promisify(db.commit.bind(db))();
                throw e;
            } else {
                await promisify(db.rollback.bind(db))();
                throw e;
            }
        }
    }

    // 個数が不足していて不成立
}
