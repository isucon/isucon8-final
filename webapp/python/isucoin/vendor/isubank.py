"""
ISUBANK API client
"""
from __future__ import annotations

import json
import urllib.parse

import requests


class IsubankError(Exception):
    msg = "Isubnak Error"


class NoUserError(IsubankError):
    msg = "no bank user"


class CreditInsufficient(IsubankError):
    msg = "credit is insufficient"


class IsuBank:
    """ISUBANK APIクライアント"""

    def __init__(self, endpoint: str, appID: str):
        """
        Arguments:
            endpoint: ISUBANK APIを利用するためのエンドポイントURI
            appID:    ISUBANK APIを利用するためのアプリケーションID
        """
        self.endpoint = endpoint
        self.appID = appID

    def Check(self, bankID: str, price: int):
        """
        Check は残高確認です

        Reserve による予約済み残高は含まれません
        """
        self._request("/check", {"bank_id": bankID, "price": price})

    def Reserve(self, bankID: str, price: int):
        """仮決済(残高の確保)を行います"""
        res = self._request("/reserve", {"bank_id": bankID, "price": price})
        return res["reserve_id"]

    def Commit(self, reserveIDs: typing.List[int]):
        """
        Commit は決済の確定を行います

        正常に仮決済処理を行っていればここでエラーになることはありません
        """
        self._request("/commit", {"reserve_ids": reserveIDs})

    def Cancel(self, reserveIDs: typing.List[int]):
        self._request("/cancel", {"reserve_ids": reserveIDs})

    def _request(self, path: str, data: dict) -> dict:
        url = urllib.parse.urljoin(self.endpoint, path)
        body = json.dumps(data)
        headers = {
            "Content-Type": "application/json",
            "Authorization": "Bearer " + self.appID,
        }

        try:
            res = requests.post(url, data=body, headers=headers)
            body = res.json()
        except Exception:
            raise IsubankError(f"{path!r} failed")

        # 参照実装に合わせて 200 以外は全部エラーとして扱う
        if res.status_code == 200:
            return body

        err = body.get("error")
        if res.status_code != 200:
            if err == "bank_id not found":
                raise NoUserError
            if err == "credit is insufficient":
                raise CreditInsufficient
            raise IsubankError(f"{path} failed: status={res.status_code} body={body}")
