from __future__ import annotations

from datetime import datetime
from dataclasses import dataclass, asdict
from isubank import CreditInsufficient
import bcrypt
import MySQLdb

from . import settings


class NoOrderForTrade(Exception):
    msg = "no order for trade"


@dataclass
class Trade:
    id: int
    amount: int
    price: int
    created_at: datetime


@dataclass
class CandlestickData:
    time: datetime
    open: int
    close: int
    high: int
    low: int


def _get_trade(db, query, *args):
    cur = db.cursor()
    cur.execute(query, args)
    row = cur.fetchone()
    if row is None:
        return None
    return Trade(*row)


def get_trade_by_id(db, id: int) -> typing.Optional[Trade]:
    return _get_trade(db, "SELECT * FROM trade WHERE id = %s", id)


def get_latest_trade(db) -> typing.Optional[Trade]:
    return _get_trade(db, "SELECT * FROM trade ORDER BY id DESC")


def get_candlestic_data(db, mt: datetime, tf: str) -> typing.List[CandlestickData]:
    query = """
        SELECT m.t, a.price, b.price, m.h, m.l
        FROM (
            SELECT
                STR_TO_DATE(DATE_FORMAT(created_at, %s), %s) AS t,
                MIN(id) AS min_id,
                MAX(id) AS max_id,
                MAX(price) AS h,
                MIN(price) AS l
            FROM trade
            WHERE created_at >= %s
            GROUP BY t
        ) m
        JOIN trade a ON a.id = m.min_id
        JOIN trade b ON b.id = m.max_id
        ORDER BY m.t
    """
    cur = db.cursor()
    cur.execute(query, (tf, "%Y-%m-%d %H:%i:%s", mt))
    return [CandlestickData(*r) for r in cur]


def has_trade_chance_by_order(db, order_id: int) -> bool:
    order = get_order_by_id(db, order_id)

    lowest = get_lowest_sell_order(db)
    if not lowest:
        return False
    highest = get_highest_buy_order(db)
    if not highest:
        return False

    if order.type == "buy" and lowest.price <= order.price:
        return True
    if order.type == "sell" and order.price <= highest.price:
        return True

    return False


def _reserve_order(db, order, price: int) -> int:
    bank = settings.get_isubank(db)
    p = order.amount * price
    if order.type == "buy":
        p = -p

    try:
        return bank.Reserve(order.user.bank_id, p)
    except CreditInsufficient as e:
        cancelOrder(db, order, "reserve_failed")
        settings.send_log(
            db,
            order.type + ".error",
            {
                "error": e.msg,
                "user_id": order.user_id,
                "amount": order.amount,
                "price": price,
            },
        )
        raise


def _commit_reserved_order(
    db, order: Order, targets: List[Order], reserve_ids: List[int]
):
    cur = db.cursor()
    cur.execute(
        "INSERT INTO trade (amount, price, created_at) VALUES (%s, %s, NOW(6))",
        (order.amount, order.price),
    )
    trade_id = cur.lastrowid
    settings.send_log(
        db,
        "trade",
        {"trade_id": trade_id, "price": order.price, "amount": order.amount},
    )

    for o in targets + [order]:
        cur.execute(
            "UPDATE orders SET trade_id = %s, closed_at = NOW(6) WHERE id = %s",
            (trade_id, o.id),
        )
        settings.send_log(
            db,
            o.type + ".trade",
            {
                "order_id": o.id,
                "price": order.Price,
                "amount": o.amount,
                "user_id": o.user_id,
                "trade_id": trade_id,
            },
        )

    bank = settings.get_isubank(db)
    bank.Commit(reserve_ids)


def try_trade(db, order_id: int):
    order = orders.get_open_order_by_id(db, order_id)

    rest_amount = order.amount
    unit_price = order.price
    reserves = [_reserve_order(db, order, unit_price)]

    try:
        if order.type == "buy":
            query = "SELECT * FROM orders WHERE type = %s AND closed_at IS NULL AND price <= %s ORDER BY price ASC, created_at ASC, id ASC"
            args = "sell", order.price
        else:
            query = "SELECT * FROM orders WHERE type = %s AND closed_at IS NULL AND price >= %s ORDER BY price DESC, created_at DESC, id DESC"
            args = "buy", order.price
        cur = db.cursor()
        cur.execute(query, args)

        target_orders = [orders.Order(*r) for r in cur]

        for to in target_orders:
            try:
                orders.get_open_order_by_id(db, to.ID)
            except orders.OrderAlreadyClosed:
                continue
            if to.amount > rest_amount:
                continue
            try:
                rid = _reserve_order(db, to, unit_price)
            except CreditInsufficient:
                continue

            reserves.append(rid)
            targets.append(to)
            rest_amount -= to.amount
            if rest_amount == 0:
                break

        if rest_amount > 0:
            raise NoOrderForTrade

        _commit_reserved_order(db, order, tragets, reserves)
        reserves.clear()
    finally:
        if reserves:
            bank = settings.get_isubank(db)
            bank.Cancel(reserves)
