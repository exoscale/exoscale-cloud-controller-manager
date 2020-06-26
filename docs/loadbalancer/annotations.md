## service.beta.kubernetes.io/exoscale-loadbalancer-zone
Specifies a zone for the Load Balancer. it is mandatory.

>note: The zone of your Load balancer 
must be like the zone of your instancepool

The possible values are `bg-sof-1`, `ch-dk-2`, `ch-gva-2`, `de-fra-1`, `de-muc-1`.

## service.beta.kubernetes.io/exoscale-loadbalancer-id
Enables fast retrievals of Load Balancer.

If no ID is specified, we retrieve the Load Balancer by name and add the ID in the annotations.

## service.beta.kubernetes.io/exoscale-loadbalancer-name
Specifies a custom name for the Load Balancer. Existing Load Balancer will be renamed. 

If no custom name is specified, we choose a default name composed as follows `nlb-{serviceUID}`.

## service.beta.kubernetes.io/exoscale-loadbalancer-description
Specifies a description for the Load Balancer. Existing Load Balancers will be renamed. 

If no description is specified, we choose a default description composed as follows `kubernetes load balancer {serviceName}`.

## service.beta.kubernetes.io/exoscale-loadbalancer-service-strategy
Specifies which strategy the Load Balancer Service should use.

The possible values are `roud-robin`, `source-hash`.
Defaults to `round-robin`.

## service.beta.kubernetes.io/exoscale-loadbalancer-service-protocol
Specifies which protocol the Load Balancer Service should use.

The possible values are `tcp`, `udp`.
Defaults to `tcp`.

## service.beta.kubernetes.io/exoscale-loadbalancer-service-id
Enables fast retrievals of Load Balancer Service.

If no ID is specified, we retrieve the Load Balancer Service by name and add the ID in the annotations.

## service.beta.kubernetes.io/exoscale-loadbalancer-service-name
Specifies a custom name for the Load Balancer Service. Existing Load Balancer Service will be renamed. 

If no custom name is specified, we choose a default name composed as follows `nlb-service-{serviceUID}`.

## service.beta.kubernetes.io/exoscale-loadbalancer-service-description
Specifies a description for the Load Balancer Service. Existing Load Balancer Service will be renamed. 

If no description is specified, we choose a default description composed as follows `kubernetes load balancer service {serviceName}`.

## service.beta.kubernetes.io/exoscale-loadbalancer-service-instancepool-id
Specifies a instancepool ID for the Load Balancer. it is mandatory.

## service.beta.kubernetes.io/exoscale-loadbalancer-service-healthcheck-mode
Specifies which mode the Load Balancer Service Health Check should use.

The possible values are `tcp`, `http`.
Defaults to `tcp`.

## service.beta.kubernetes.io/exoscale-loadbalancer-service-http-healthcheck-uri
Specifies the URI that is used by the "http" Load Balancer Service Health Check.

Defaults to `/`.

## service.beta.kubernetes.io/exoscale-loadbalancer-service-healthcheck-interval
Specifies the number of seconds between two consecutive health checks. 

The value must be between `5s` and `300s`.

Defaults to `10s`.

## service.beta.kubernetes.io/exoscale-loadbalancer-service-healthcheck-timeout
Specifies the number of seconds will wait for a response until marking a health check as failed. 

The value must be `5s` and should `not be bigger than Interval`. 

Defaults to `5s`.

## service.beta.kubernetes.io/exoscale-loadbalancer-service-healthcheck-retries
Specifies the number of retries before considering a service failed.

The value must be between `1` and `20`. 

Defaults to `1`.