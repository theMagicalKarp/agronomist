## What

Agronomist is a Kuberntes pod autoscaler which uses OPA to determine deployment scale.

![](https://i.imgur.com/ydIcnT7.png)

## Installing

### Install Metrics-Server (if not installed)

```
helm install metrics-server stable/metrics-server
```

### Install Agronomist CRDs

```
helm install agronomist-crd ./helm/agronomist-crds
```

### Install Agronomist

```
helm install agronomist ./helm/agronomist -n kube-system
```

## Example

This is a very basic example to demonstrate the capabilities.

```YAML
apiVersion: agronomist.io/v1
kind: ScalingPolicy
metadata:
  name: foo
spec:
  rego: |
    package main

    utilization = util {
        total_limit := sum([parseunit(cpu) | cpu := input.pods[_].spec.containers[_].resources.limits.cpu])
        total_usage := sum([parseunit(cpu) | cpu := input.podMetrics[_].containers[_].usage.cpu])

        util := total_usage/total_limit * 100.0
    }

    scale = result {
        utilization > 75.0
        result := count(input.pods) + 1
    }

    scale = result {
        75.0 >= utilization
        utilization > 25.0
        result := count(input.pods)
    }

    scale = result {
        25.0 >= utilization
        result := count(input.pods) - 1
    }

  deployment: "my-deployment"
  min: 3
  max: 10

  maxStepUp: 2
  maxStepDown: 2

  upDelay: 30
  downDelay: 30

  interval: 5
```

In this example we target the deployment `my-deployment` and scale it based on the results from the `scale` function from the following rego file.

```rego
package main

utilization = util {
    total_limit := sum([parseunit(cpu) | cpu := input.pods[_].spec.containers[_].resources.limits.cpu])
    total_usage := sum([parseunit(cpu) | cpu := input.podMetrics[_].containers[_].usage.cpu])

    util := total_usage/total_limit * 100.0
}

scale = result {
    utilization > 75.0
    result := count(input.pods) + 1
}

scale = result {
    75.0 >= utilization
    utilization > 25.0
    result := count(input.pods)
}

scale = result {
    25.0 >= utilization
    result := count(input.pods) - 1
}
```


## Rego Builtins

* `parseunit` parses/converts kuberntes units to canonical units

## TODO

* Make unit tests/Linting/Setup CI
* Use real logging library/prometheus metrics
* Cleanup CRDs/ Use real structs in memory
* Structure Code Better
* Create better docs/examples
* Allow usage of External/Custom metrics
* Allow inclusion of other resources for determining scaling
* Publish Helm Chart
* Come up with better naming schema for ScalingPolicyStatus
* Create tooling for running locally/testing rego queries
* Better error handling for scripts which fail (loopback errors to ScalingPolicy/ScalingPolicyStatus resource)
* Support workloads other than deployments?
* Determine if should/can be deployed per namespace instead of per cluster

