from __future__ import annotations

import time

from .orders import *
from .settings import *
from .trades import *
from .users import *


def init_benchmark(db):
    cur = db.cursor()
    cur.execute("DELETE FROM orders WHERE created_at >= '2018-10-16 10:00:00'")
    cur.execute("DELETE FROM trade  WHERE created_at >= '2018-10-16 10:00:00'")
    cur.execute("DELETE FROM user   WHERE created_at >= '2018-10-16 10:00:00'")
