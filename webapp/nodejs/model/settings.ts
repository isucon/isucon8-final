import { dbQuery } from '../db';
import { IsuBank } from '../vendor/isubank';
import { IsuLogger } from '../vendor/isulogger';

const BANK_ENDPOINT = 'bank_endpoint';
const BANK_APPID = 'bank_appid';
const LOG_ENDPOINT = 'log_endpoint';
const LOG_APPID = 'log_appid';

export async function setSetting(k: string, v: string) {
    await dbQuery(
        'INSERT INTO setting (name, val) VALUES (?, ?) ON DUPLICATE KEY UPDATE val = VALUES(val)',
        [k, v]
    );
}

export async function getSetting(k: string) {
    const [result] = await dbQuery('SELECT val FROM setting WHERE name = ?', [
        k,
    ]);
    return result;
}

export async function getIsubank() {
    const endpoint = await getSetting(BANK_ENDPOINT);
    const appid = await getSetting(BANK_APPID);
    return new IsuBank(endpoint, appid);
}

async function getLogger() {
    const endpoint = await getSetting(LOG_ENDPOINT);
    const appid = await getSetting(LOG_APPID);
    return new IsuLogger(endpoint, appid);
}

export async function sendLog(
    tag: string,
    v: {
        error?: string;
        amount?: number;
        price?: number;
        orderId?: number;
        userId?: number;
        tradeId?: number;
        reason?: string;
        bankId?: string;
        name?: string;
    }
) {
    const logger = await getLogger();
    await logger.send(tag, v);
}
