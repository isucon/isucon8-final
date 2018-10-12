from __future__ import annotations

import os, sys
sys.path.append(os.path.dirname(__file__) + "/vendor")

import contextlib
import time
import flask
import MySQLdb

from . import model


# port   = os.environ.get("ISU_APP_PORT", "5000")
dbhost = os.environ.get("ISU_DB_HOST", "127.0.0.1")
dbport = os.environ.get("ISU_DB_PORT", "3306")
dbuser = os.environ.get("ISU_DB_USER", "root")
dbpass = os.environ.get("ISU_DB_PASSWORD", "")
dbname = os.environ.get("ISU_DB_NAME", "isucoin")
public = os.environ.get("ISU_PUBLIC_DIR", "public")

app = flask.Flask(__name__, static_url_path="/", static_folder=public)
app.secret_key = "tonymoris"


_dbconn = None


def get_dbconn():
    # NOTE: get_dbconn() is not thread safe.  Don't use threaded server.
    global _dbconn

    if _dbconn is None:
        _dbconn = MySQLdb.connect(
            host=dbhost,
            port=int(dbport),
            user=dbuser,
            password=dbpass,
            database=dbname,
            charset="utf8mb4",
            autocommit=True,
        )

    return _dbconn


def error_json(code: int, msg):
    resp = flask.jsonify({"code": code, "err": str(msg)})
    resp.headers["X-Content-Type-Options"] = "nosniff"
    resp.status_code = code
    return resp


@app.errorhandler(Exception)
def errohandler(err):
    return error_json(500, err)


@app.before_request
def before_request():
    user_id = flask.session.get("user_id")
    if user_id is None:
        flask.g.current_user = None
        return

    user = model.user.get_user_by_id(user_id)
    if user is None:
        flask.session.clear()
        return error_json(404, "セッションが切断されました")

    flask.g.current_user = user


@contextlib.contextmanager
def transaction():
    conn = get_dbconn()
    conn.begin()
    try:
        yield conn
    except:
        conn.rollback()
    else:
        conn.commit()


@app.route("/initialize")
def initialize():
    with transaction() as db:
        model.init_benchmark(db)

        for k in ("bank_endpoint", "bank_appid", "log_endpoint", "log_appid"):
            v = flask.request.form.get(k)
            model.set_setting(db, k, v)
    return flask.jsonify({})


@app.route("/signup")
def signup():
    req = flask.request
    name = req.form["name"]
    bank_id = req.form["bank_id"]
    password = req.form["password"]

    if not (name and bank_id and password):
        return error_json(400, "all parameters are required")

    try:
        with transaction() as db:
            model.user.signup(db, name, bank_id, password)
    except model.user.BankUserNotFound as e:
        return error_json(404, e.msg)
    except model.user.BankUserConflict as e:
        return error_json(409, e.msg)

    return flask.jsonify({})


@app.route("/signin")
def signin():
    req = flask.request
    bank_id = req.form["bank_id"]
    password = req.form["password"]

    if not (bank_id and password):
        return error_json(400, "all parameters are required")

    db = get_dbconn()
    try:
        user_id, name = model.user.login(db, bank_id, password)
    except model.user.UserNotFound as e:
        # TODO: 失敗が多いときに403を返すBanの仕様に対応
        return error_json(404, e.msg)

    flask.session["user_id"] = user_id
    return flask.jsonify(id=user_id, name=name)


@app.route("/signout")
def signout():
    flask.session.clear()
    return flask.jsonify({})


@app.route("/info")
def info():
    res = {}
    db = get_dbconn()
    cursor = flask.request.args["cursor"]
    last_trade_id = 0
    lt = 0

    if cursor:
        try:
            last_trade_id = int(cursor)
        except ValueErorr as e:
            app.logger.exception(f"failed to parse cursor ({cursor!r})")
        if last_trade_id > 0:
            trade = model.trade.get_trade_by_id(db, last_trade_id)
            if trade:
                lt = trade.created_at

    latest_trade = model.trade.get_latest_trade(db)
    res["cursor"] = latest_trade.id

    user = flask.g.current_user
    if user:
        orders = model.orders.get_orders_by_userid_and_lasttradeid(
            db, user.id, last_trade_id
        )
        for o in orders:
            model.orders.fech_order_relation(o)

        res["traded_orders"] = orders  # jsonify?

    now = time.time()  # localtime?

    # todo: chart
    res["lowest_sell_price"] = 100  # todo
    res["highest_buy_price"] = 1000  # todo

    # TODO: trueにするとシェアボタンが有効になるが、アクセスが増えてヤバイので一旦falseにしておく
    res["enable_share"] = False

    return flask.jsonify(res)


@app.route("/orders")
def orders():
    user = flask.g.current_user
    if user is None:
        return error_json(401, "Not authenticated")

    db = get_dbconn()
    orders = model.orders.get_orders_by_userid(db, user.id)
    for o in orders:
        model.orders.fech_order_relation(o)

    return flask.jsonify(orders)


@app.route("/orders", methods=("POST",))
def add_order():
    user = flask.g.current_user
    if user is None:
        return error_json(401, "Not authenticated")

    amount = int(flask.request.form["amount"])
    price = int(flask.request.form["price"])
    type = flask.request.form["type"]
    print(f"add_order: amount={amount}, price={price}, type={type}")

    try:
        with transaction() as db:
            order = model.orders.add_order(db, type, user.id, amount, price)
    except (model.order.InvalidParameter, model.order.CreditInsufficient) as e:
        return error_json(400, e.msg)

    db = get_dbconn()
    trade_chance = model.trade.has_trade_chance(db, order.id)
    if trade_chance:
        try:
            model.trade.run_trade(db)
        except Exception:  # トレードに失敗してもエラーにはしない
            app.logger.exception("run_trade failed")

    return flask.jsonify(id=order.id)


@app.route("/order/<int:order_id>", methods=("DELETE",))
def delete_order(order_id):
    user = flask.g.current_user
    if user is None:
        return error_json(401, "Not authenticated")

    try:
        with transaction() as db:
            model.orders.delete_order(db, user.id, order_id, "canceled")
    except (model.orders.OrderNotFound, model.order.OrderAlreadyClosed) as e:
        error_json(404, e.msg)

    return flask.jsonify(id=order_id)
