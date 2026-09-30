import json

import pytest

from receptorctl.socket_interface import ReceptorControl


@pytest.mark.parametrize(
    "tlsclient,signwork,optional_params",
    [
        (None, False, {}),
        ("client1", True, {"tlsclient": "client1", "signwork": "true"}),
    ],
)
def test_adopt_work_serializes_optional_parameters(
    monkeypatch, tlsclient, signwork, optional_params
):
    controller = ReceptorControl("unix:///unused")
    connect_calls = []
    written = []
    response = {"result": "Adopted", "unitid": "local-unit"}

    monkeypatch.setattr(controller, "connect", lambda: connect_calls.append(True))
    monkeypatch.setattr(controller, "writestr", written.append)
    monkeypatch.setattr(controller, "read_and_parse_json", lambda: response)

    result = controller.adopt_work("node1", "remote-unit", tlsclient=tlsclient, signwork=signwork)

    assert result is response
    assert connect_calls == [True]
    assert len(written) == 1
    assert written[0].endswith("\n")
    assert json.loads(written[0]) == {
        "command": "work",
        "subcommand": "adopt",
        "node": "node1",
        "unitid": "remote-unit",
        **optional_params,
    }
