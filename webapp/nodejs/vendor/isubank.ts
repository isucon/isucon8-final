import urljoin from 'url-join';
import fetch, { Response } from 'node-fetch';

class IsubankError extends Error {
    constructor(message?: string) {
        super(message || 'Isubank Error');
        Object.setPrototypeOf(this, IsubankError.prototype);
    }
}

class NoUserError extends Error {
    constructor() {
        super('no bank user');
        Object.setPrototypeOf(this, NoUserError.prototype);
    }
}

class CreditInsufficient extends Error {
    constructor() {
        super('credit is insufficient');
        Object.setPrototypeOf(this, CreditInsufficient.prototype);
    }
}

// ISUBANK APIクライアント
export class IsuBank {
    /**
     * @param endpoint ISUBANK APIを利用するためのエンドポイントURI
     * @param appID ISUBANK APIを利用するためのアプリケーションID
     */
    constructor(public endpoint: string, public appID: string) {}

    /**
     * Check は残高確認です
     * Reserve による予約済み残高は含まれません
     */
    async check(bankID: string, price: number) {
        await this.request('/check', { bank_id: bankID, price: price });
    }

    /**
     * 仮決済(残高の確保)を行います
     */
    async reserve(bankID: string, price: number): Promise<number> {
        const res = await this.request('/reserve', {
            bank_id: bankID,
            price: price,
        });
        return res.reserve_id;
    }

    /**
     * Commit は決済の確定を行います
     * 正常に仮決済処理を行っていればここでエラーになることはありません
     */
    async commit(reserveIDs: number[]): Promise<void> {
        await this.request('/commit', { reserve_ids: reserveIDs });
    }

    async cancel(reserveIDs: number[]): Promise<void> {
        await this.request('/cancel', { reserve_ids: reserveIDs });
    }

    private async request(path: string, data: object) {
        const url = urljoin(this.endpoint, path);
        const body = JSON.stringify(data);
        const headers = {
            'Content-Type': 'application/json',
            Authorization: 'Bearer ' + this.appID,
        };

        let res: Response;
        try {
            res = await fetch(url, {
                method: 'POST',
                body,
                headers,
            });
        } catch (e) {
            throw new IsubankError(`${path} failed`);
        }

        if (res.status === 200) {
            return res.json();
        }

        const { error } = await res.json();
        if (error === 'bank_id not found') {
            throw new NoUserError();
        }
        if (error === 'credit is insufficient') {
            throw new CreditInsufficient();
        }
        throw new IsubankError(
            `${path} failed: status=${res.status} body=${body}`
        );
    }
}
