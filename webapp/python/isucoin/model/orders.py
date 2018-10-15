from __future__ import annotations

from dataclasses import dataclass, asdict
import datetime
import isubank

from . import users, trades, settings


class OrderAlreadyClosed(Exception):
    msg = "order is already closed"


class OrderNotFound(Exception):
    msg = "order not found"


class CreditInsufficient(Exception):
    msg = "銀行の残高が足りません"


@dataclass
class Order:
    id: int
    type: str
    user_id: int
    amount: int
    price: int
    closed_at: typing.Optional[str]
    trade_id: int
    created_at: datetime.datetime
    user: typing.Optional[users.User] = None
    trade: typing.Optional[trades.Trade] = None

    def __init__(
        self, id, type, user_id, amount, price, closed_at, trade_id, created_at
    ):
        if isinstance(type, bytes):
            type = type.decode()
        self.id = id
        self.type = type
        self.user_id = user_id
        self.amount = amount
        self.price = price
        self.closed_at = closed_at
        self.trade_id = trade_id
        self.created_at = created_at

    def to_json(self):
        data = asdict(self)
        if self.trade_id is None:
            del data["trade_id"]
        if self.user is None:
            del data["user"]
        if self.trade is None:
            del data["trade"]
        return data


def get_orders_by_userid(db, user_id: int) -> typing.List[Order]:
    c = db.cursor()
    c.execute(
        "SELECT * FROM orders WHERE user_id = %s AND (closed_at IS NULL OR trade_id IS NOT NULL) ORDER BY created_at ASC",
        (user_id,),
    )
    return [Order(*r) for r in c]


def get_orders_by_userid_and_lasttradeid(
    db, user_id: int, trade_id: int
) -> typing.List[Order]:
    c = db.cursor()
    c.execute(
        "SELECT * FROM orders WHERE user_id = %s AND trade_id IS NOT NULL AND trade_id > %s ORDER BY created_at ASC",
        (user_id, trade_id),
    )
    return [Order(*r) for r in c]


def _get_one_order(db, query, *args):
    c = db.cursor()
    c.execute(query, args)
    row = c.fetchone()
    if row is None:
        return None
    return Order(*row)


def get_order_by_id(db, id: int) -> Order:
    return _get_one_order(db, "SELECT * FROM orders WHERE id = %s", id)


def get_order_by_id_with_lock(db, id: int) -> Order:
    order = _get_one_order(db, "SELECT * FROM orders WHERE id = %s FOR UPDATE", id)
    order.user = users.get_user_by_id_with_lock(db, order.user_id)
    return order


def get_open_order_by_id(db, id: int) -> Order:
    order = get_order_by_id_with_lock(db, id)
    if order.closed_at is not None:
        raise OrderAlreadyClosed
    return order


def get_lowest_sell_order(db) -> Order:
    return _get_one_order(
        db,
        "SELECT * FROM orders WHERE type = %s AND closed_at IS NULL ORDER BY price ASC, created_at ASC LIMIT 1",
        "sell",
    )


def get_highest_buy_order(db) -> Order:
    return _get_one_order(
        db,
        "SELECT * FROM orders WHERE type = %s AND closed_at IS NULL ORDER BY price DESC, created_at ASC LIMIT 1",
        "buy",
    )


def fetch_order_relation(db, order: Order):
    order.user = users.get_user_by_id(db, order.user_id).to_json()
    if order.trade_id:
        order.trade = asdict(trades.get_trade_by_id(db, order.trade_id))


def add_order(db, ot: str, user_id: int, amount: int, price: int) -> Order:
    if amount <= 0 or price <= 0:
        raise ValueError
    user = users.get_user_by_id_with_lock(db, user_id)

    bank = settings.get_isubank(db)

    if ot == "buy":
        total = price * amount
        try:
            bank.Check(user.bank_id, total)
        except isubank.CreditInsufficient as e:
            settings.send_log(
                db,
                "buy.error",
                {"error": e.msg, "user_id": user_id, "amount": amount, "price": price},
            )
            raise CreditInsufficient
    elif ot == "sell":
        pass
    else:
        raise ValueError

    cur = db.cursor()
    cur.execute(
        "INSERT INTO orders (type, user_id, amount, price, created_at) VALUES (%s, %s, %s, %s, NOW(6))",
        (ot, user_id, amount, price),
    )
    id = cur.lastrowid

    settings.send_log(
        db,
        ot + ".order",
        {"order_id": id, "user_id": user_id, "amount": amount, "price": price},
    )

    return get_order_by_id(db, id)


def delete_order(db, user_id: int, order_id: int, reason: str):
    user = users.get_user_by_id_with_lock(db, user_id)
    order = get_order_by_id_with_lock(db, order_id)

    if order is None:
        raise OrderNotFound
    if order.user_id != user.id:
        raise OrderNotFound
    if order.closed_at is not None:
        raise OrderAlreadyClosed

    return cancel_order(db, order, reason)


def cancel_order(db, order: Order, reason: str):
    cur = db.cursor()
    cur.execute("UPDATE orders SET closed_at = NOW(6) WHERE id = %s", (order.id,))
    settings.send_log(
        db,
        order.type + ".delete",
        {"order_id": order.id, "user_id": order.user_id, "reason": reason},
    )
