# CRUD

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
Using images built locally under minikube for simplicity (skips any registry).
Pre-load the DB image with an init-file containing the schema.

```
alias 'kubectl' 'minikube kubectl --'
eval (minikube docker-env)
docker build --target krud-psql-img -t krud-psql-img .
docker build --target krud-http-img -t krud-http-img .

kubectl get all
kubectl logs -fl app=krud-http-deployment

kubectl port-forward service/krud-psql-service 2345:2345
kubectl port-forward service/krud-http-service 8080:8080
minikube service --url service-name krud-http-service

kubectl delete deployment --all
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