#!/bin/bash

PID_FILE="terraform-${TARGET_CLUSTER}/ccm.pid"

exoscale_ccm_start() {
	if [ -f "$PID_FILE" ]; then
		echo "CCM is already running (PID file is present: $PID_FILE)"
		exit 1
	fi

	echo " > Exoscale-CCM: Building initial environment"
	. "terraform-${TARGET_CLUSTER}/.env"
	
	echo " > Exoscale-CCM: Starting"
	go run ../cmd/exoscale-cloud-controller-manager/main.go \
		--kubeconfig=$CCM_KUBECONFIG \
		--authentication-kubeconfig=$CCM_KUBECONFIG \
		--authorization-kubeconfig=$CCM_KUBECONFIG \
		--cloud-config=cloud-config.conf \
		--leader-elect=true \
		--allow-untagged-cloud  \
		--v=3 > ccm.log 2>&1 &

	CCM_PID=$!
	echo " > Exoscale-CCM: Started (PID=${CCM_PID})"
	echo "$CCM_PID" > "$PID_FILE"
}

exoscale_ccm_kill() {
	if [ -f "$PID_FILE" ]; then
		CCM_PID="$(cat $PID_FILE)"
		echo " > Exoscale-CCM: Killing (PID=${CCM_PID})"
		pkill -P "$CCM_PID"
		echo " > Exoscale-CCM: Killed"
		rm "$PID_FILE"
	fi
}