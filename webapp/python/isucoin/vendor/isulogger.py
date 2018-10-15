"""
ISULOG client
"""
from __future__ import annotations

import json
import time
import urllib.parse

import requests


class IsuLogger:
    def __init__(self, endpoint, appID):
        self.endpoint = endpoint
        self.appID = appID

    def send(self, tag, data):
        self._request(
            "/send",
            {
                "tag": tag,
                "time": time.strftime("%Y-%m-%dT%H:%M:%S+09:00"),
                "data": data,
            },
        )

    def _request(self, path, data):
        url = urllib.parse.urljoin(self.endpoint, path)
        body = json.dumps(data)
        headers = {
            "Content-Type": "application/json",
            "Authorization": "Bearer " + self.appID,
        }

        res = requests.post(url, data=body, headers=headers)
        res.raise_for_status()
