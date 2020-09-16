import urljoin from 'url-join';
import fetch from 'node-fetch';

export class IsuLogger {
    constructor(public endpoint: string, public appID: string) {}

    async send(tag: string, data: object) {
        this.request('/send', {
            tag,
            time: new Date().toISOString().replace(/\.[0-9]{3}Z/, '+09:00'),
            data,
        });
    }

    private async request(path: string, data: object) {
        const url = urljoin(this.endpoint, path);
        const body = JSON.stringify(data);
        const headers = {
            'Content-Type': 'application/json',
            Authorization: 'Bearer ' + this.appID,
        };

        const res = await fetch(url, { body, headers, method: 'POST' });
        if (res.status >= 300) {
            throw new Error(
                `failed isulogger request ${res.statusText} ${
                    res.status
                } ${await res.text()}`
            );
        }
    }
}
