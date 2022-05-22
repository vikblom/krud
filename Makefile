REV := $(shell git describe --tags --dirty 2>/dev/null || true)

# TODO: Retrieve this from gcloud command?
ARTIFACT_REPO := "europe-north1-docker.pkg.dev/valid-climber-350112/kube-images"

.PHONY: images

images: Dockerfile
	docker build --target krud-psql -t ${ARTIFACT_REPO}/krud-psql:latest .
	docker push ${ARTIFACT_REPO}/krud-psql:latest
	docker build --target krud-http -t ${ARTIFACT_REPO}/krud-http:latest .
	docker push ${ARTIFACT_REPO}/krud-http:latest
