import { dbQuery } from '../db';

export async function initBenchmark() {
    await dbQuery(
        `DELETE FROM orders WHERE created_at >= '2018-10-16 10:00:00'`
    );
    await dbQuery(
        `DELETE FROM trade  WHERE created_at >= '2018-10-16 10:00:00'`
    );
    await dbQuery(
        `DELETE FROM user   WHERE created_at >= '2018-10-16 10:00:00'`
    );
}
