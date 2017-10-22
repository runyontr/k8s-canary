# K8s Canary

This tutorial will walk through how to perform a canary deployment on [Kubernetes](https://kubernetes.io/). 
When updating an application, a slow rollout will allow for monitoring of traffic to the new
deployment and allow a rollback of the deployment with minimal impact to operations.






## Kubernetes Services

A [Service](https://kubernetes.io/docs/concepts/services-networking/service/) is an abstraction of the concept of pods. 
since pods can be created and destroyed, a service provides a single host:port to talk to the pods
configured for this service.  Services are created with a set of label selectors, and all requests
made to the service will be balanced to pods running with the configured label selectors.


There is no restriction that the pods behind a service need to come from the same deployment, and this will
allow us to create a canary deployment to test an update before performing a full update.



# The App

The applications we'll use to show the canary deployment are located in the `app` folder. The application
 uses an interface to abstract out the implementation of the AppInfo service.  
 
 
The application is supposed to provide information about how the application has been deployed on
Kubernetes.  It looks for environment variables containing metadata about the application and 
labels on the container and provides them back as a json message.


```go
package models 

type AppInfo struct {
	PodName string //Extracts the MY_POD_NAME environment variable
	AppName string //Extracts the value for the app label 
	Namespace string //Extracts the MY_POD_NAMESPACE environment variable
	Release string //Extracts the value for the release label
	Labels  map[string]string //Contains other labels for app
}
```

## App Structure

### Models

The `app/models` folder contains the data transfer objects for the application.


### Service

The `app/service` folder contains the interface definition and the implemenattions of the interface that 
extract the AppInfo from the runtime.  This folder contains three implementations of the interface.



### Transport

The `app/transport` folder contains the [go-kit](https://github.com/go-kit/kit) code to host the 
interface implementation as a service.
The `http.Handler` returned from `MakeInfoServiceHandler` is hosted to translate a request to `/v1/appinfo` to
the `GetAppInfo` function of the interface implementation and serialize the response back.

 
 
### Dockerfiles

There are three dockerfiles in the `app` folder that correspond to images built with different implementations 
of the interface.  The following table shows which structs are being used in which Dockerfile, what tag these
images are pushed up with, and a brief description of what should be expected when running the application.



 Struct   |Dockerfile | Docker Image| Description |
 ---|---|---|---|
 appInfoBaseline | app/Dockerfile.v1 | `runyonsolutions/appinfo:1` | Does not return Namespace value.
 appInfoBroken | app/Dockerfile.v2 |`runyonsolutions/appinfo:2` | Returns an error
 appInfoWithNamespace | app/Dockerfile.v3 |`runyonsolutions/appinfo:3` | populates Namespace value correctly
 


 
 In practice, the application being deployed would probably not have three separate implementations
  of the same  interface, but might represent iterations made to one implementation. 
   The use of three different structs simplifies showing the code running in each image.
 
 The application layout does show how easily it would be to change how a service functions and interchange different storage
 systems for basic CRUD applications, or different implementations of analytics. 
 
 


## Deployment

As a baseline, we assume there is a Kubernetes deployment named appinfo that was deployed from the 
`deployment/appinfo.yaml` file.  This deployment can be created via

```
kubectl create -f deployment/appinfo.yaml
```

The deployment is very basic except for two additional configuration.  First is  [Exposing Pod Information through 
Environment Variables](https://kubernetes.io/docs/tasks/inject-data-application/environment-variable-expose-pod-information/). 
These are created from the yaml in [Deployment YAML](deployment/appinfo.yaml)

```yaml
        env:
        - name: MY_POD_NAME
          valueFrom:
            fieldRef:
              fieldPath: metadata.name
        - name: MY_POD_NAMESPACE
          valueFrom:
            fieldRef:
              fieldPath: metadata.namespace
```

Which create an environment variable `MY_POD_NAME` that contains the pod name and `MY_POD_NAMESPACE` 
which contains the namespace where the pod is running.

The second customization is [Exposing pod labels into a container](https://kubernetes-v1-4.github.io/docs/user-guide/downward-api/).
which creates a file at `/etc/labels` containing the pods labels.


Now get the name of the pod that was deployed:

```
kubectl get pods -l app=appinfo
```

output:

```
NAME                       READY     STATUS    RESTARTS   AGE
appinfo-567b989978-vvzlg   1/1       Running   0          19s

```

## Talking to the deployment

Now we can port forward localhost:8080 to the pod's container port 8080 via

```
POD_NAME=$(kubectl get pods -l app=appinfo -o jsonpath="{.items[0].metadata.name}")
kubectl port-forward $POD_NAME 8080:8080
```
output
```
Forwarding from 127.0.0.1:8080 -> 80
Forwarding from [::1]:8080 -> 80
```


Open a new terminal and we can send an HTTP request:


```
curl -s localhost:8080/v1/appinfo | jq .
```
and get a response like
```
{
  "PodName": "appinfo-567b989978-vvzlg",
  "AppName": "appinfo",
  "Namespace": "",
  "Release": "stable",
  "Labels": {
    "pod-template-hash": "1236545534"
  }
}

``` 

In a new terminal run this command to be constantly refreshing the AppInfo, which will be useful to show
the real time updates in the runtime in the following section.

```
 while true; do clear; curl -s localhost:8080/v1/appinfo | jq . ; sleep 1; done;
```


### Changing labels


To see how the `/etc/labels` file is updated dynamically, we can add a new label to the pod and see
the output of our while loop get adjusted in real time

```
kubectl label pods $POD_NAME newlabel=realtime
``` 



Switch back to the first terminal to stop the port forwarding with CTRL+C.


## Create Service

A service provides load balancing to a selection of pods based on a particular label. 
 The label we're going
to filter by is `app=appinfo`.  To see the pods that satisfy this, run

```
kubectl get pods -l app=appinfo
```


The service defined in `deployment/appinfo-service.yaml` has the label selector defined as `app=appinfo`. 
 This will create
a load balancer (service) that routes requests to pods with the label `app=appinfo`. 

```
kubectl create -f deployment/appinfo-service.yaml
```

and see it

```
kubectl get svc appinfo
```

output:

```
NAME      CLUSTER-IP    EXTERNAL-IP   PORT(S)          AGE
appinfo   10.32.0.232   <nodes>       8080:30753/TCP   14s
```

There will now be an environment variables in all running containers that start after the creation
of this service:

```
APPINFO_SERVICE_PORT=8080
APPINFO_PORT_8080_TCP_PORT=8080
APPINFO_PORT=tcp://10.32.0.232:8080
APPINFO_PORT_8080_TCP_ADDR=10.32.0.232
```

which will allow all pods running in the cluster to have the connection information for this service readily available.



### Accessing the Service

The type of service we created was [NodePort](https://kubernetes.io/docs/concepts/services-networking/service/#type-nodeport).
  
  When this type of service is created, each node will proxy requests to a specific port on the node
   (the same port on each node) to the configured port on the pod (8080 for our application).  
 
 To obtain the port on the node, run
 ```
 NODE_PORT=$(kubectl get svc appinfo \
   --output=jsonpath='{range .spec.ports[0]}{.nodePort}')
 ```
 
 
### Cloud Firewall
  When running on a cloud provider, a firewall rule might need to be added to allow TCP traffic into the
  nodes at the service port.  To enable traffic in Google Cloud, the following command will open
  traffic to the node port.
```
gcloud compute firewall-rules create appinfo-service \
  --allow=tcp:${NODE_PORT} \
  --network ${KUBERNETES_NETWORK_NAME}
```

where `${KUBERNETES_NETWORK_NAME}` is the name of the network Kubernetes is deployed on.


Now if `${EXTERNAL_IP}` is the public address of one of the nodes in the cluster, then
the service is available at `http://${EXTERNAL_IP}:${NODE_PORT}`


## Connecting

Similar to the while loop monitoring the port forwarded traffic of a particular pod in 
[Talking to the Deployment](#talking-to-the-deployment), the loop

```
 while true; do clear; curl -s ${EXTERNAL_IP}:${NODE_PORT}/v1/appinfo | jq . ; sleep 1; done;
```

will show the output of the service.  In order to monitor the deployments in the following section, 
this command should be run in a new terminal.

# Deploy an Update (Canary)

Looking at the output of the loop, we see that the namespace field is not being populated correctly by the
deployment.  One proposed (failed) solution is captured in the implementation `appInfoBroken`, which
simulates a developers failed attempt at fixing the issue.  As new software can sometimes contain bugs, a slow
rollout of the new version of the software will allow for minimal impact if there is an issue.



To simulate a real deployment,  we should have the currently deployed application scaled to handle
the current traffic.  When looking at how large to scale the current deployment (n), its helpful to 
understand the system's SLOs.  The new canary deployment will be getting 1/(n+1) of the traffic
going to the service, and reducing the canary's load by increasing the value of n
 lower the impact to the error budget when
things go wrong.

For this tutorial, we scale to 3 replicas.

```
kubectl scale --replicas=3 deployment appinfo
deployment "appinfo" scaled
```

```
kubectl get pods -l app=appinfo
NAME                       READY     STATUS    RESTARTS   AGE
appinfo-567b989978-6hbm8   1/1       Running   0          15s
appinfo-567b989978-q58nt   1/1       Running   0          15s
appinfo-567b989978-vvzlg   1/1       Running   0          14m
```

Looking at the output of the while loop look should now show the responses coming from pods with different names. 
 Additionally,
if any labels were applied to the first pod in [Chaning Labels](#changing-labels), the newly created pods
 will not have those labels.  This should be seen as responses will have different sets of labels.


Now we are ready to deploy our broken canary deployment:


```
kubectl create -f deployment/canary-broken.yaml 
deployment "appinfo-canary-broken" created
```

Looking at the running pods, we can now see 4 pods with the `app=appinfo` label:

```
kubectl get pods -l app=appinfo
NAME                                    READY     STATUS    RESTARTS   AGE
appinfo-567b989978-6hbm8                1/1       Running   0          10m
appinfo-567b989978-q58nt                1/1       Running   0          10m
appinfo-567b989978-vvzlg                1/1       Running   0          25m
appinfo-canary-broken-c66665c44-7cfk6   1/1       Running   0          19s
```


Now looking at the loop in [Connection](#connecting) should have about 1/4 of the response coming back with a message of
```json
{"error": "something went wrong"}
```
 and the other 3/4 of response should be responding as before.


### Monitoring

This is when application monitoring (e.g. Prometheus/Grafana) would be able to split the metrics between canary
 and stable pods and show any difference in performance.  A future iteration of this tutorial will demonstrate
 the performance differences with a monitoring solution.


### Rollback

For this demo, the while loop will be used to demonstrate the health of the system, and since
  there are errors being returned in some responses, a rollback of the deployment is required:
  

```
kubectl delete -f deployment/canary-broken.yaml
```

At this point, the broken pod should be terminating:

```
kubectl get pods -l app=appinfo
NAME                                    READY     STATUS        RESTARTS   AGE
appinfo-567b989978-6hbm8                1/1       Running       0          12m
appinfo-567b989978-q58nt                1/1       Running       0          12m
appinfo-567b989978-vvzlg                1/1       Running       0          26m
appinfo-canary-broken-c66665c44-7cfk6   1/1       Terminating   0          2m
```

and responses will return to being valid 100% of the time. In most shops, this would be enough to justify 
a [Post Mortem](https://landing.google.com/sre/book/chapters/postmortem-culture.html) and improve any
testing process prior to being considered for a release.
                                                             




## Fix

After figuring out the issue, a new fix has been created and is ready to be rolled out.  Following a similar
process, a new canary deployment is created:

```
kubectl create -f deployment/canary-fixed.yaml 
deployment "appinfo-canary-fixed" created
```



Looking at the output of the while loop now shows about 1/4 of the requests have the correct namespace value (`default`), and
are reporting the `release=canary` label.  


### Acceptance
At the point the team is willing to accept the new version formally, the configuration on the 
 stable app would need to be updated to the docker image of the canary deployment:


```
kubectl set image deployment/appinfo appinfo-containers=runyonsolutions/appinfo:3
```

Since the image is being update this should fire off a rolling update of the deployment and new pods should be created

```
kubectl get pods -l app=appinfo
NAME                                    READY     STATUS              RESTARTS   AGE
appinfo-567b989978-6hbm8                1/1       Terminating         0          16m
appinfo-567b989978-q58nt                1/1       Terminating         0          16m
appinfo-567b989978-vvzlg                1/1       Running             0          30m
appinfo-84d5cf794d-9dpfz                1/1       Running             0          5s
appinfo-84d5cf794d-dd67v                1/1       Running             0          10s
appinfo-84d5cf794d-lq45c                0/1       ContainerCreating   0          2s
appinfo-95bccb844-jnfzn                 0/1       Terminating         3          1m
appinfo-canary-fixed-744f96dc75-zbxr9   1/1       Running             0          2m
```

Describing any of the newly created pods should show the update image.  The monitoring loop in [Connection](#connecting)
should should show labels `release=stable` having the namespace value correctly set.

Finally, we need to clean up the canary app.

```
kubectl delete -f deployment/canary-fixed.yaml
```

Now all pods running behind the service are updated.

