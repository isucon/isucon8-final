import log4js from 'log4js';
import bcrypt from 'bcrypt';
import { dbQuery } from '../db';
import { getIsubank, sendLog } from './settings';
const logger = log4js.getLogger();

export class BankUserNotFound extends Error {
    constructor() {
        super('bank user not found');
        Object.setPrototypeOf(this, BankUserNotFound.prototype);
    }
}

export class BankUserConflict extends Error {
    constructor() {
        super('bank user conflict');
        Object.setPrototypeOf(this, BankUserConflict.prototype);
    }
}

export class UserNotFound extends Error {
    constructor() {
        super('user not found');
        Object.setPrototypeOf(this, UserNotFound.prototype);
    }
}

export class User {
    constructor(
        public id: number,
        public bank_id: string,
        public name: string,
        public password: string,
        public created_at: Date
    ) {
        this.bank_id = bank_id.toString();
        this.name = name.toString();
        this.password = password.toString();
    }
}

export async function getUserById(id: number): Promise<User> {
    const [r] = await dbQuery('SELECT * FROM user WHERE id = ?', [id]);
    const { id: _id, bank_id, name, password, created_at } = r;
    return new User(_id, bank_id, name, password, created_at);
}

export async function getUserByIdWithLock(id: number) {
    const [r] = await dbQuery('SELECT * FROM user WHERE id = ? FOR UPDATE', [
        id,
    ]);
    const { id: _id, bank_id, name, password, created_at } = r;
    return new User(_id, bank_id, name, password, created_at);
}

export async function signup(name: string, bankId: string, password: string) {
    const bank = await getIsubank();
    // bank_idの検証
    try {
        await bank.check(bankId, 0);
    } catch (e) {
        logger.error(`failed to check bank_id (${bankId})`);
        throw new BankUserNotFound();
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
        bank_id: bankId,
        user_id: userId,
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

    const { id: _id, bank_id, name, password: _password, created_at } = row;
    const user = new User(_id, bank_id, name, _password, created_at);
    if (!(await bcrypt.compare(password, user.password))) {
        throw new UserNotFound();
    }

    sendLog('signin', { user_id: user.id });
    return user;
}
