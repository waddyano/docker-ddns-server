#/bin/bash
RELEASED_TAG=localhost/dyndns
TESTING_TAG=localhost/testing

if podman image exists "$RELEASED_TAG"; then 
    podman untag "$RELEASED_TAG"
fi

if ! podman image exists "$TESTING_TAG"; then 
    echo "$TESTING_TAG does not exist"
    exit 1  
fi

podman tag $TESTING_TAG $RELEASED_TAG
podman rmi $TESTING_TAG