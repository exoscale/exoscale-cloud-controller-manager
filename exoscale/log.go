package exoscale

import (
	"k8s.io/klog/v2"
)

func infof(format string, args ...interface{}) {
	klog.Infof("exoscale-ccm: "+format, args...)
}

func debugf(format string, args ...interface{}) {
	klog.V(3).Infof("exoscale-ccm: "+format, args...)
}
