import { dbQuery } from '../db';
import { getIsubank, sendLog } from './settings';
import { getTradeById, Trade } from './trades';
import { getUserById, getUserByIdWithLock, User } from './users';

export class OrderAlreadyClosed extends Error {
    constructor() {
        super('order is already closed');
        Object.setPrototypeOf(this, OrderAlreadyClosed.prototype);
    }
}

class OrderNotFound extends Error {
    constructor() {
        super('order not found');
        Object.setPrototypeOf(this, OrderNotFound.prototype);
    }
}

export class CreditInsufficient extends Error {
    constructor() {
        super('銀行の残高が足りません');
        Object.setPrototypeOf(this, CreditInsufficient.prototype);
    }
}

export class Order {
    public user?: User | null;
    public trade?: Trade | null;
    constructor(
        public id: number,
        public type: string,
        public user_id: number,
        public amount: number,
        public price: number,
        public closed_at: Date,
        public trade_id: number,
        public created_at: Date
    ) {
        this.type = type.toString();
    }
}

export async function getOrdersByUserId(userId: number): Promise<Order[]> {
    const result = await dbQuery(
        `SELECT * FROM orders WHERE user_id = ? AND (closed_at IS NULL OR trade_id IS NOT NULL) ORDER BY created_at ASC`,
        [userId]
    );
    return result.map(
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
}

export async function getOrdersByUserIdAndLasttradeid(
    userId: number,
    tradeId: number
): Promise<Order[]> {
    const result = await dbQuery(
        'SELECT * FROM orders WHERE user_id = ? AND trade_id IS NOT NULL AND trade_id > ? ORDER BY created_at ASC',
        [userId, tradeId]
    );
    return result.map(
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
}

async function getOneOrder(
    query: string,
    ...args: any[]
): Promise<Order | null> {
    const [result] = await dbQuery(query, args);
    if (!result) return null;
    return new Order(
        result.id,
        result.type,
        result.user_id,
        result.amount,
        result.price,
        result.closed_at,
        result.trade_id,
        result.created_at
    );
}

export async function getOrderById(id: number): Promise<Order | null> {
    return getOneOrder('SELECT * FROM orders WHERE id = ?', id);
}

async function getOrderByIdWithLock(id: number): Promise<Order | null> {
    const order = await getOneOrder(
        'SELECT * FROM orders WHERE id = ? FOR UPDATE',
        id
    );
    if (!order) return null;
    order.user = await getUserByIdWithLock(order.user_id);
    return order;
}

export async function getOpenOrderById(id: number): Promise<Order | null> {
    const order = await getOrderByIdWithLock(id);
    if (order?.closed_at) {
        throw new OrderAlreadyClosed();
    }
    return order;
}

export async function getLowestSellOrder(): Promise<Order | null> {
    return getOneOrder(
        'SELECT * FROM orders WHERE type = ? AND closed_at IS NULL ORDER BY price ASC, created_at ASC LIMIT 1',
        'sell'
    );
}

export async function getHighestBuyOrder(): Promise<Order | null> {
    return getOneOrder(
        'SELECT * FROM orders WHERE type = ? AND closed_at IS NULL ORDER BY price DESC, created_at ASC LIMIT 1',
        'buy'
    );
}

export async function fetchOrderRelation(order: Order): Promise<void> {
    order.user = await getUserById(order.user_id);
    if (order.trade_id) {
        order.trade = await getTradeById(order.trade_id);
    }
}

export async function addOrder(
    ot: string,
    userId: number,
    amount: number,
    price: number
): Promise<Order> {
    if (amount <= 0 || price <= 0) {
        throw new Error('value error');
    }
    const user = await getUserByIdWithLock(userId);
    const bank = await getIsubank();
    if (ot === 'buy') {
        const total = price * amount;
        try {
            await bank.check(user.bank_id, total);
        } catch (e) {
            await sendLog('buy.error', {
                error: e.message,
                user_id: userId,
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
        order_id: insertId,
        user_id: userId,
        amount: amount,
        price: price,
    });
    return (await getOrderById(insertId)) as Order;
}

export async function deleteOrder(
    userId: number,
    orderId: number,
    reason: string
): Promise<void> {
    const user = await getUserByIdWithLock(userId);
    const order = await getOrderByIdWithLock(orderId);
    if (!order) {
        throw new OrderNotFound();
    }
    if (order.user_id !== user.id) {
        throw new OrderNotFound();
    }
    if (order.closed_at) {
        throw new OrderAlreadyClosed();
    }

    return cancelOrder(order, reason);
}

export async function cancelOrder(order: Order, reason: string): Promise<void> {
    await dbQuery('UPDATE orders SET closed_at = NOW(6) WHERE id = ?', [
        order.id,
    ]);
    await sendLog(order.type + '.delete', {
        order_id: order.id,
        user_id: order.user_id,
        reason: reason,
    });
}
