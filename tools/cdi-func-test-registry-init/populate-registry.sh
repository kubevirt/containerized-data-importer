#!/bin/sh

# Populate regisry host with disk images encapsulated inside container images. 
# Disk images are taken from /tmp/shared/images directory populated by cdi-func-test-registry-init
# Container images are built with buildah 

CONTAINER_DISK_IMAGE=${CONTAINER_DISK_IMAGE:-quay.io/kubevirt/container-disk-v1alpha}
ARCHITECTURES="${ARCHITECTURES:-amd64,arm64,s390x}"
: "${MULTIARCH_IMAGES:=tinycoreqcow2}"
MULTIARCH_IMAGES=($MULTIARCH_IMAGES)

#images args
IMAGES_SRC=$1        #path to files to be encapsulated in docker image
IMAGES_CTR=$2        #path to directories with Dockerfile per file

#registry args
REGISTRY_HOST=$3     #host name of docker registry
REGISTRY_PORT=$4     #port of docker registry
REGISTRY_TLS="--tls-verify=false"

#health args
HEALTH_PATH=$5       #path f or health indicators - shared between popuplate and registry-host containers
HEALTH_PERIOD=$6
READYNESS_PATH=$7    #path f or readyness indicators - shared between popuplate and registry-host containers
READYNESS_PERIOD=$8

function  ready {
    echo "readiness"
    touch $1
}

#healthy if registry is accessible and can return the list of images
function health {
    echo "health"
    registry=$registry_host":"$registry_port
    status=$?
    curl -k -X GET https://$registry/v2/_catalog &> /dev/null
    if [ $status -eq 0 ]; then 
       touch $1
    else 
       echo "registry is inaccessible"
    fi
}

function imageList {
    registry=$registry_host":"$registry_port
    echo curl -k -X GET https://$registry/v2/_catalog
}

in_array() {
    local item match="$1"
    shift
    for item; do
        [[ "$item" == "$match" ]] && return 0
    done
    return 1
}

#Convert all images to docker build consumable format
DIR="-dir"
DOCKERFILE="Dockerfile"
VMIMAGEFILENAME="disk.img"

function prepareImages {
   images_in=$1
   images_out=$2 
   
   rm -rf $images_out
   mkdir -p $images_out
   cp  $images_in* $images_out

   cd $images_out

   for FILENAME in $(ls); do
        mkdir -p $FILENAME$DIR
        cp  $FILENAME $FILENAME$DIR

        local IMAGE_LOCATION 
        if [[ $FILENAME == *"gz"* ]]; then
            IMAGE_LOCATION="/"
        else
            IMAGE_LOCATION="/disk"
        fi

        FILE=$FILENAME$DIR"/"$DOCKERFILE
        /bin/cat  >$FILE <<-EOF
                FROM $CONTAINER_DISK_IMAGE
                COPY $FILENAME $IMAGE_LOCATION
                ENV TEST_KUBEVIRT_IO_TEST=testvalue
                ENV TEST_KUBEVIRT_IO_EXISTING=somethingelse
                ENV INSTANCETYPE_KUBEVIRT_IO_DEFAULT_INSTANCETYPE=test-instancetype
                ENV INSTANCETYPE_KUBEVIRT_IO_DEFAULT_INSTANCETYPE_KIND=VirtualMachineClusterInstancetype
                ENV INSTANCETYPE_KUBEVIRT_IO_DEFAULT_PREFERENCE=test-preference
                ENV INSTANCETYPE_KUBEVIRT_IO_DEFAULT_PREFERENCE_KIND=VirtualMachineClusterPreference
EOF

        rm $FILENAME
  done
}

function error {
	if [ "$1" -ne "0" ]; then
           echo "Exiting on error"
	   exit -1
	fi
}

#Iterate over all images build them and push them into cdi registry
function pushImages {
   images=$1
   registry_host=$2
   registry_port=$3
   registry_tls=$4
   registry=$registry_host":"$registry_port

   shopt -s nullglob
   for IMAGEDIR in *$DIR; do
        cd $IMAGEDIR
        declare -l FILE
        FILE=$(ls | grep -v $DOCKERFILE)
        IMAGENAME=${FILE//.}
        echo "building image "$IMAGENAME
        if in_array "$IMAGENAME" "${MULTIARCH_IMAGES[@]}"; then
          buildah bud --platform $ARCHITECTURES --manifest $IMAGENAME":latest" $images"/"$IMAGEDIR"/"
          error $?
          echo "pushing image "$IMAGENAME" to registry-service: "$registry
          buildah manifest push --all $registry_tls  $IMAGENAME":latest" "docker://"$registry"/"$IMAGENAME
        else 
          buildah bud -t $IMAGENAME":latest" $images"/"$IMAGEDIR"/"
          error $?
          echo "pushing image "$IMAGENAME" to registry-service: "$registry
          buildah push $registry_tls  $IMAGENAME":latest" "docker://"$registry"/"$IMAGENAME
        fi
        error $?
        cd ../
   done
}

update-ca-trust

# Avoid 'overlay' is not supported over overlayfs error
sed -i 's,driver =.*,driver = "vfs",' /etc/containers/storage.conf

#building using buildah requires a properly installed shadow-utils package (which in turn requires SETFCAP)
rpm --restore shadow-utils 2>/dev/null

#start health beat
health $HEALTH_PATH $HEALTH_PERIOD &

#prepare and poush images
prepareImages $IMAGES_SRC $IMAGES_CTR
pushImages $IMAGES_CTR $REGISTRY_HOST $REGISTRY_PORT $REGISTRY_TLS

#mark container as ready
ready $READYNESS_PATH $READYNESS_PERIOD &

#sleep forever
trap : TERM INT
sleep infinity & wait


