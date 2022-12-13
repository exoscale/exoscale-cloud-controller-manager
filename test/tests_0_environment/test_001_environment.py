import json
import os

import pytest

from helpers import (
    TEST_CCM_TYPE,
    TEST_CCM_EXEC_TERRAFORM,
    execForeground,
    kubectl,
    exocli,
)


@pytest.mark.environment
def test_type():
    if TEST_CCM_TYPE not in ["sks", "kubeadm"]:
        pytest.exit(f"Invalid test type ({TEST_CCM_TYPE})")


@pytest.mark.environment
def test_exoscale_credentials():
    api_key = os.getenv("EXOSCALE_API_KEY", "unset")
    if api_key == "unset" or api_key == "":
        pytest.exit("Missing/empty EXOSCALE_API_KEY environment variable")
    api_secret = os.getenv("EXOSCALE_API_SECRET", "unset")
    if api_secret == "unset" or api_secret == "":
        pytest.exit("Missing/empty EXOSCALE_API_SECRET environment variable")


@pytest.mark.environment
def test_executable_terraform(test, logger):
    (iExit, sStdOut, sStdErr) = execForeground(
        [
            TEST_CCM_EXEC_TERRAFORM,
            "version",
            "-json",
        ],
        pyexit=True,
    )
    output = json.loads(sStdOut)
    version = output["terraform_version"]
    logger.info(f"Terraform version: {version}")


@pytest.mark.environment
def test_executable_kubectl(test, logger):
    (iExit, sStdOut, sStdErr) = kubectl(
        [
            "--output=json",
            "version",
            "--client=true",
        ],
        pyexit=True,
    )
    output = json.loads(sStdOut)
    version = output["clientVersion"]["gitVersion"]
    logger.info(f"kubectl version: {version}")


@pytest.mark.environment
def test_executable_exocli(test, logger):
    (iExit, sStdOut, sStdErr) = exocli(
        [
            "version",
        ],
        pyexit=True,
    )
    logger.info(f"Exoscale CLI version: {sStdOut}")
