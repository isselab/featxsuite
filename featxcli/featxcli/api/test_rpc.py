import pytest
from featxcli.api.rpc import Rpc

# Fixture to set up the Rpc instance
@pytest.fixture
def rpcCall():
    return Rpc()

def test_encodeMessage():
    pass

def test_decodeMessage():
    pass