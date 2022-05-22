# KRUD

## Stack

### Database

Postgresql running in a Docker container.

`github.com/jackc/pgx/v4` with the `stblib` package so the interface is plain ol' `database/sql`.

### HTTP

Gorilla mux but otherwise `http` and `httptest`.

Probably missing some tricks and best practices to make to code smaller/simpler.

### Testing

Standard library.

Database tests require an env var set to some psql DB.
Just putting sqlite in a `t.TmpDir()` would be much simpler.

`httptest` for endpoint tests. Low coverage due to lack of time.
Is there a good way of comparing expected vs. actual response vs. actual db change?

## TODO

- Use anon. struct with json tags for API?
- Validate incoming Content Type.
- HTTPS.
- Log stmts across handlers and db.
- k8s secrets.
- k8s persistence.
- Better "Update" API.

## Minikube

### Develop w/o a docker registry

Use images built locally under minikube for simplicity (skips any registry).
Pre-load the DB image with an init-file containing the schema.

```
minikube start
alias 'kubectl' 'minikube kubectl --'

kubectl get all
kubectl logs -fl app=krud-http-deployment

kubectl port-forward service/krud-psql-service 2345:2345
kubectl port-forward service/krud-http-service 8080:8080
minikube service --url service-name krud-http-service

kubectl delete deployment --all
```

### Using minikube and GCP docker registry

Pulling down images proved to be quite tricky.
If `docker login` works that can be made into a k8s secret:
```
kubectl create secret generic regcred \
    --from-file=.dockerconfigjson=$HOME/.docker/config.json \
    --type=kubernetes.io/dockerconfigjson
kubectl patch serviceaccount default \
    -p '{"imagePullSecrets": [{"name": "regcred"}]}'
kubectl get serviceaccount default -o yaml
```

## Useful Links

- https://hub.docker.com/_/postgres

- https://www.vinaysahni.com/best-practices-for-a-pragmatic-restful-api

- https://github.com/cockroachdb/copyist

- https://medium.com/@saumya.ranjan/how-to-create-a-rest-api-in-golang-krud-operation-in-golang-a7afd9330a7b

- https://drstearns.github.io/tutorials/gomiddleware/

- https://blog.questionable.services/article/guide-logging-middleware-go/

- https://learning-cloud-native-go.github.io/docs/index

- https://levelup.gitconnected.com/deploying-dockerized-golang-api-on-kubernetes-with-postgresql-mysql-d190e27ac09f

- https://marukhno.com/running-go-application-in-kubernetes/

- https://medium.com/google-cloud/kubernetes-nodeport-vs-loadbalancer-vs-ingress-when-should-i-use-what-922f010849e0
