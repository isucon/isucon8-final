from __future__ import annotations

import time

from .orders import *
from .settings import *
from .trades import *
from .users import *


def init_benchmark(db):
    """
    前回の10:00:00+0900までのデータを消す

    本戦当日は2018-10-20T10:00:00+0900 固定だが、他の時間帯にデータ量を揃える必要がある
    """
    stop = time.strftime("%Y-%m-%d 10:00:00", time.localtime(time.time() - 10 * 3600))

    cur = db.cursor()
    cur.execute("DELETE FROM orders WHERE created_at >= %s", (stop,))
    cur.execute("DELETE FROM trade  WHERE created_at >= %s", (stop,))
    cur.execute("DELETE FROM user   WHERE created_at >= %s", (stop,))
