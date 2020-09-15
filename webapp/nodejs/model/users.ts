import log4js from 'log4js';
import bcrypt from 'bcrypt';
import { dbQuery } from '../db';
import { getIsubank, sendLog } from './settings';
const logger = log4js.getLogger();

export class BankUserNotFound extends Error {
    constructor() {
        super('bank user not found');
    }
}

export class BankUserConflict extends Error {
    constructor() {
        super('bank user conflict');
    }
}

export class UserNotFound extends Error {
    constructor() {
        super('user not found');
    }
}

export class User {
    constructor(
        public id: number,
        public bankId: string,
        public name: string,
        public password: string,
        public createdAt: string
    ) {}
}

export async function getUserById(id: number): Promise<User> {
    const [r] = await dbQuery('SELECT * FROM user WHERE id = ?', [id]);
    const [_id, bankId, name, password, createdAt] = r;
    return new User(_id, bankId, name, password, createdAt);
}

export async function getUserByIdWithLock(id: number) {
    const [r] = await dbQuery('SELECT * FROM user WHERE id = ? FOR UPDATE', [
        id,
    ]);
    const [_id, bankId, name, password, createdAt] = r;
    return new User(_id, bankId, name, password, createdAt);
}

export async function signup(name: string, bankId: string, password: string) {
    const bank = await getIsubank();

    // bank_idの検証
    try {
        await bank.check(bankId, 0);
    } catch (e) {
        logger.error(`failed to check bank_id (${bankId})`);
        throw e;
    }

    const hpass = await bcrypt.hash(password, await bcrypt.genSalt());
    let userId: number;
    try {
        const result = await dbQuery(
            'INSERT INTO user (bank_id, name, password, created_at) VALUES (?, ?, ?, NOW(6))',
            [bankId, name, hpass]
        );
        userId = result.insertId;
    } catch (e) {
        throw new BankUserConflict();
    }
    sendLog('signup', {
        bankId,
        userId,
        name,
    });
}

export async function login(bankId: string, password: string) {
    const [row] = await dbQuery('SELECT * FROM user WHERE bank_id = ?', [
        bankId,
    ]);
    if (!row) {
        throw new UserNotFound();
    }
    const [_id, _bankId, name, _password, createdAt] = row;
    const user = new User(_id, _bankId, name, _password, createdAt);

    if (!(await bcrypt.compare(password, user.password))) {
        throw new UserNotFound();
    }

    sendLog('signin', { userId: user.id });
    return user;
}
