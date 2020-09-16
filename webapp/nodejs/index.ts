import express from 'express';
import log4js from 'log4js';
import session from 'express-session';
import { promisify } from 'util';
import path from 'path';
import bodyParser from 'body-parser';
import morgan from 'morgan';
import { initBenchmark } from './model';
import { setSetting } from './model/settings';
import {
    addOrder,
    deleteOrder,
    fetchOrderRelation,
    getHighestBuyOrder,
    getLowestSellOrder,
    getOrdersByUserId,
    getOrdersByUserIdAndLasttradeid,
    Order,
} from './model/orders';
import { transaction } from './db';
import {
    BankUserConflict,
    BankUserNotFound,
    getUserById,
    login,
    signup,
    User,
} from './model/users';
import {
    getCandlesticData,
    getLatestTrade,
    getTradeById,
    hasTradeChanceByOrder,
    runTrade,
} from './model/trades';

declare global {
    namespace Express {
        export interface Request {
            currentUser?: {
                id: number;
                bank_id: string;
                name: string;
                password: string;
                created_at: Date;
            };
        }
    }
}

const logger = log4js.getLogger();

const app = express();
app.use(morgan('tiny'));

const PUBLIC_DIR = process.env.ISU_PUBLIC_DIR || 'public';

/*
 * ISUCON用初期データの基準時間です
 * この時間以降のデータはinitializeで削除されます
 */
const BASE_TIME = new Date(2018, 10 - 1, 16, 10, 0, 0);

function sendError(res: express.Response, code: number, msg: string) {
    res.set('X-Content-Type-Options', 'nosniff');
    res.status(code).json({ code, msg });
}

app.use(express.static(PUBLIC_DIR));
app.use(bodyParser.urlencoded({ extended: false }));
app.use(session({ secret: 'tonymoris' }));

app.use(async function beforeRequest(req, res, next) {
    const userId = req.session?.userId;
    if (!userId) {
        req.currentUser = undefined;
        next();
        return;
    }
    const user = await getUserById(userId);
    if (!user) {
        await promisify(req.session!.destroy)();
        return sendError(res, 404, 'セッションが切断されました');
    }
    req.currentUser = user;
    next();
});

app.get('/', (req, res) => {
    res.sendFile(path.join(PUBLIC_DIR, 'index.html'));
});

app.post('/initialize', async (req, res) => {
    await transaction(async () => {
        await initBenchmark();
    });

    for (const k of [
        'bank_endpoint',
        'bank_appid',
        'log_endpoint',
        'log_appid',
    ]) {
        const v = req.body[k];
        await setSetting(k, v);
    }

    res.json({});
});

app.post('/signup', async (req, res, next) => {
    const { name, bank_id, password } = req.body;
    if (!(name && bank_id && password)) {
        sendError(res, 400, 'all parameters are required');
        return;
    }

    try {
        await transaction(async () => {
            await signup(name, bank_id, password);
        });
    } catch (e) {
        if (e instanceof BankUserNotFound) {
            sendError(res, 404, e.message);
            return;
        }
        if (e instanceof BankUserConflict) {
            sendError(res, 409, e.message);
            return;
        }
        next(e);
        return;
    }
    res.json({});
});

app.post('/signin', async (req, res) => {
    const { bank_id, password } = req.body;
    if (!(bank_id && password)) {
        sendError(res, 400, 'all parameters are required');
        return;
    }

    let user;
    try {
        user = await login(bank_id, password);
    } catch (e) {
        return sendError(res, 404, e.message);
    }

    req.session!.userId = user.id;
    res.json({ id: user.id, name: user.name });
});

app.post('/signout', async (req, res) => {
    await promisify(req.session!.destroy)();
    res.json({});
});

app.get('/info', async (req, res) => {
    const info: any = {};
    const { cursor } = req.query;
    let lastTradeId = 0;
    let lt = null;
    if (cursor) {
        try {
            lastTradeId = parseInt(cursor as string);
        } catch (e) {
            logger.error(`failed to parse cursor (${cursor})`);
        }
        if (lastTradeId > 0) {
            const trade = await getTradeById(lastTradeId);
            if (trade) {
                lt = trade.created_at;
            }
        }
    }

    const latestTrade = await getLatestTrade();
    info.cursor = latestTrade!.id;
    const user = req.currentUser;
    if (user) {
        const orders = await getOrdersByUserIdAndLasttradeid(
            user.id,
            lastTradeId
        );
        for (const o of orders) {
            await fetchOrderRelation(o);
        }
        info.traded_orders = orders;
    }

    let fromT = new Date(BASE_TIME.getTime() - 300 * 1000);
    if (lt && lt > fromT) {
        fromT = new Date(lt);
    }
    info.chart_by_sec = await getCandlesticData(fromT, '%Y-%m-%d %H:%i:%s');

    fromT = new Date(BASE_TIME.getTime() - 300 * 60 * 1000);
    info.chart_by_min = await getCandlesticData(fromT, '%Y-%m-%d %H:%i:00');

    fromT = new Date(BASE_TIME.getTime() - 48 * 60 * 60 * 1000);
    info.chart_by_hour = await getCandlesticData(fromT, '%Y-%m-%d %H:00:00');

    const lowestSellOrder = await getLowestSellOrder();
    if (lowestSellOrder) {
        info.lowest_sell_price = lowestSellOrder.price;
    }

    const highestBuyOrder = await getHighestBuyOrder();
    if (highestBuyOrder) {
        info.highest_buy_price = highestBuyOrder.price;
    }

    info.enable_share = false;
    res.json(info);
});

app.get('/orders', async (req, res) => {
    const user = req.currentUser;
    if (!user) {
        return sendError(res, 401, 'Not authenticated');
    }

    const orders = await getOrdersByUserId(user.id);
    for (const o of orders) {
        await fetchOrderRelation(o);
    }

    res.json(orders);
});

app.post('/orders', async (req, res) => {
    const user = req.currentUser;
    if (!user) {
        return sendError(res, 401, 'Not authenticated');
    }

    const amount = parseInt(req.body.amount);
    const price = parseInt(req.body.price);
    const type = req.body.type;

    let order: Order | undefined;
    try {
        await transaction(async () => {
            order = await addOrder(type, user.id, amount, price);
        });
    } catch (e) {
        return sendError(res, 400, e.message);
    }

    if (!order) {
        return sendError(res, 400, 'hogehoge');
    }
    const tradeChance = await hasTradeChanceByOrder(order.id);

    if (tradeChance) {
        try {
            await runTrade();
        } catch (e) {
            // トレードに失敗してもエラーにはしない
            logger.error('run_trade failed');
        }
    }

    res.json({ id: order!.id });
});

app.delete('/order/:id', async (req, res) => {
    const { id } = req.params;
    const orderId = parseInt(id);
    const user = req.currentUser;
    if (!user) {
        return sendError(res, 401, 'Not authenticated');
    }

    try {
        await transaction(async () => {
            await deleteOrder(user.id, orderId, 'canceled');
        });
    } catch (e) {
        return sendError(res, 404, e.message);
    }

    res.json({ id: orderId });
});

app.use(function errorHandler(
    err: Error,
    _req: express.Request,
    res: express.Response,
    _next: express.NextFunction
) {
    logger.error('FAIL');
    sendError(res, 500, err.message);
});

const PORT = process.env.ISU_APP_PORT || 5000;
app.listen(PORT, function listeningListener() {
    console.log(`listening on ${PORT}`);
});
