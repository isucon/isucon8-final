from __future__ import annotations

import logging
from datetime import datetime
from dataclasses import dataclass
import bcrypt
import MySQLdb

from . import settings


class BankUserNotFound(Exception):
    msg = "bank user not found"


class BankUserConflict(Exception):
    msg = "bank user conflict"


class UserNotFound(Exception):
    msg = "user not found"


@dataclass
class User:
    id: int
    bank_id: str
    name: str
    password: bytes
    created_at: datetime

    def __init__(self, id, bank_id, name, password, created_at):
        if isinstance(bank_id, bytes):
            bank_id = bank_id.decode()
        if isinstance(name, bytes):
            name = name.decode()

        self.id = id
        self.bank_id = bank_id
        self.name = name
        self.password = password
        self.created_at = created_at

    def to_json(self):
        return {"id": self.id, "name": self.name}


def get_user_by_id(db, id: int) -> User:
    cur = db.cursor()
    cur.execute("SELECT * FROM user WHERE id = %s", (id,))
    r = cur.fetchone()
    if r is not None:
        r = User(*r)
    return r


def get_user_by_id_with_lock(db, id: int) -> User:
    cur = db.cursor()
    cur.execute("SELECT * FROM user WHERE id = %s FOR UPDATE", (id,))
    r = cur.fetchone()
    if r is not None:
        r = User(*r)
    return r


def signup(db, name: str, bank_id: str, password: str):
    bank = settings.get_isubank(db)

    # bank_idの検証
    try:
        bank.Check(bank_id, 0)
    except Exception:
        logging.exception(f"failed to check bank_id ({bank_id})")
        raise BankUserNotFound

    hpass = bcrypt.hashpw(password.encode(), bcrypt.gensalt(10))

    cur = db.cursor()
    try:
        cur.execute(
            "INSERT INTO user (bank_id, name, password, created_at) VALUES (%s, %s, %s, NOW(6))",
            (bank_id, name, hpass),
        )
    except MySQLdb.IntegrityError:
        raise BankUserConflict

    user_id = cur.lastrowid

    settings.send_log(
        db, "signup", {"bank_id": bank_id, "user_id": user_id, "name": name}
    )


def login(db, bank_id: str, password: str) -> User:
    cur = db.cursor()
    cur.execute("SELECT * FROM user WHERE bank_id = %s", (bank_id,))
    row = cur.fetchone()
    if not row:
        raise UserNotFound
    user = User(*row)

    if not bcrypt.checkpw(password.encode(), user.password):
        raise UserNotFound

    settings.send_log(db, "signin", {"user_id": user.id})
    return user
