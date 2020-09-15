import { dbQuery } from '../db';
import { getIsubank, sendLog } from './settings';
import { getTradeById, Trade } from './trades';
import { getUserById, getUserByIdWithLock, User } from './users';

export class OrderAlreadyClosed extends Error {
    constructor() {
        super('order is already closed');
    }
}

class OrderNotFound extends Error {
    constructor() {
        super('order not found');
    }
}

export class CreditInsufficient extends Error {
    constructor() {
        super('銀行の残高が足りません');
    }
}

export class Order {
    public user?: User;
    public trade?: Trade;
    constructor(
        public id: number,
        public type: string,
        public userId: number,
        public amount: number,
        public price: number,
        public closedAt: string,
        public tradeId: number,
        public createdAt: string
    ) {}
}

export async function getOrdersByUserId(userId: number) {
    const result = await dbQuery(
        `SELECT * FROM orders WHERE user_id = ? AND (closed_at IS NULL OR trade_id IS NOT NULL) ORDER BY created_at ASC`,
        [userId]
    );
    //@ts-ignore
    return result.map((row) => new Order(...row));
}

export async function getOrdersByUserIdAndLasttradeid(
    userId: number,
    tradeId: number
) {
    const result = await dbQuery(
        'SELECT * FROM orders WHERE user_id = ? AND trade_id IS NOT NULL AND trade_id > ? ORDER BY created_at ASC',
        [userId, tradeId]
    );
    //@ts-ignore
    return result.map((row) => new Order(...row));
}

async function getOneOrder(query: string, ...args: any[]): Promise<Order> {
    const [result] = await dbQuery(query, args);
    return result;
}

export async function getOrderById(id: number) {
    return getOneOrder('SELECT * FROM orders WHERE id = ?', id);
}

async function getOrderByIdWithLock(id: number) {
    const order = await getOneOrder(
        'SELECT * FROM orders WHERE id = ? FOR UPDATE',
        id
    );
    order.user = await getUserByIdWithLock(order.userId);
    return order;
}

export async function getOpenOrderById(id: number) {
    const order = await getOrderByIdWithLock(id);
    if (order.closedAt) {
        throw new OrderAlreadyClosed();
    }
    return order;
}

export async function getLowestSellOrder() {
    return getOneOrder(
        'SELECT * FROM orders WHERE type = ? AND closed_at IS NULL ORDER BY price ASC, created_at ASC LIMIT 1',
        'sell'
    );
}

export async function getHighestBuyOrder() {
    return getOneOrder(
        'SELECT * FROM orders WHERE type = ? AND closed_at IS NULL ORDER BY price DESC, created_at ASC LIMIT 1',
        'buy'
    );
}

export async function fetchOrderRelation(order: Order) {
    order.user = await getUserById(order.userId);
    if (order.tradeId) {
        order.trade = await getTradeById(order.tradeId);
    }
}

export async function addOrder(
    ot: string,
    userId: number,
    amount: number,
    price: number
) {
    if (amount <= 0 || price <= 0) {
        throw new Error('value error');
    }
    const user = await getUserByIdWithLock(userId);
    const bank = await getIsubank();
    if (ot === 'buy') {
        const total = price * amount;
        try {
            await bank.check(user.bankId, total);
        } catch (e) {
            await sendLog('buy.error', {
                error: e.message,
                userId: userId,
                amount: amount,
                price: price,
            });
            throw new CreditInsufficient();
        }
    } else if (ot !== 'sell') {
        throw new Error('value error');
    }

    const {
        insertId,
    } = await dbQuery(
        'INSERT INTO orders (type, user_id, amount, price, created_at) VALUES (?, ?, ?, ?, NOW(6))',
        [ot, userId, amount, price]
    );
    await sendLog(ot + '.order', {
        orderId: insertId,
        userId: userId,
        amount: amount,
        price: price,
    });

    return getOrderById(insertId);
}

export async function deleteOrder(
    userId: number,
    orderId: number,
    reason: string
) {
    const user = await getUserByIdWithLock(userId);
    const order = await getOrderByIdWithLock(orderId);
    if (!order) {
        throw new OrderNotFound();
    }
    if (order.userId !== user.id) {
        throw new OrderNotFound();
    }
    if (order.closedAt) {
        throw new OrderAlreadyClosed();
    }

    return cancelOrder(order, reason);
}

export async function cancelOrder(order: Order, reason: string) {
    await dbQuery('UPDATE orders SET closed_at = NOW(6) WHERE id = ?', [
        order.id,
    ]);
    await sendLog(order.type + '.delete', {
        orderId: order.id,
        userId: order.userId,
        reason: reason,
    });
}
