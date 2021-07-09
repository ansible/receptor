#!/bin/bash

set -e

# Global vars
tests_dirs=$(find . -name *_test.go -exec dirname {} + | uniq)
tests_files=$(find . -name *_test.go)

# Logs. Based on:
# https://github.com/containers/Demos/blob/master/building/buildah_intro/buildah_intro.sh
if [[ ${TERM} != "dumb" ]]; then
    bold=$(tput bold)
    reset=$(tput sgr0)

    red=$(tput setaf 1)
    green=$(tput setaf 2)
    yellow=$(tput setaf 3)
    blue=$(tput setaf 4)
    purple=$(tput setaf 5)
    cyan=$(tput setaf 6)
    white=$(tput setaf 7)
    grey=$(tput setaf 8)
    vivid_red=$(tput setaf 9)
    vivid_green=$(tput setaf 10)
    vivid_yellow=$(tput setaf 11)
    vivid_blue=$(tput setaf 12)
    vivid_purple=$(tput setaf 13)
    vivid_cyan=$(tput setaf 14)
fi
log() {
    if [[ $1 == *"["*"]"* ]]; then
        out=$(echo $1 | sed "s/]/]${reset}/g")
        echo "${bold}$out${reset}"
    else
        echo "${bold}$1${reset}"
    fi
}
log_bold() {
    echo "${bold}$1${reset}"
}
log_info() {
    if [[ $1 == *"["*"]"* ]]; then
        out=$(echo $1 | sed "s/]/]${reset}/g")
        echo "${vivid_purple}$out${reset}"
    else
        echo "${cyan}$1${reset}"
    fi
}
log_warning() {
    echo "${vivid_yellow}$1${reset}"
}
log_error() {
    echo "${vivid_red}$1${reset}"
}

# Helpers
function check_requirements(){
    # Check if podman is installed
    if ! command -v podman &> /dev/null
    then
        log_error "ERROR: podman is not installed!"
        exit 1
    fi

    # Check if make is installed
    if ! command -v make &> /dev/null
    then
        log_error "ERROR: make is not installed!"
        exit 1
    fi

    # Check if Makefile exists
    # TODO
}
function build_artifacts(){
    make artifacts
}

# Import main libs and vars
function f_list_dirs() {
    echo $tests_dirs | sed 's/ /\n/g'
}

function f_list_files() {
    echo $tests_files | sed 's/ /\n/g'
}

function f_run() {
    # Ensure podman is installed
    check_requirements
    
    # Build artifacts if not exist yet
	if ! test -f "./artifacts/receptor"; then 
        log_warning "# Artifacts not found..."
        build_artifacts
    fi

    # Get container image tag
    container_image_tag=$(cat Makefile | grep "^CONTAINER_IMAGE_TAG_BASE=" | cut -d'=' -f 2)

    # Fix possible permission issue
    chmod +x ./artifacts/receptor

    # Run test command inside the container
    RUN_CMD='
        PATH=$PATH:/artifacts && 
        cd /source/tests && set -x &&
        go test -v '$@'
    '
	podman run -it --rm \
		-v $(pwd)/../:/source/:ro \
		-v $(pwd)/artifacts:/artifacts/:rw \
		-v receptor_go_root_cache:/root/go:rw \
		$container_image_tag bash -c "${RUN_CMD}"
}

function f_run_all() {
    for f in ${tests_files} ; do
        log_info "# START Test for '${f}'"
        $0 run $f
    done
}

function f_help() {
    echo
    cat <<- HELP_INFO
Command list:
  list-dirs    - list all available tests directories
  list-files   - list all available tests files
  run          - WIP run a specific test
  run-all      - WIP run all tests. Returns 0 if pass
  help         - show this help section
HELP_INFO
}

# Menu
if [[ $1 == "list-dirs" ]]; then
    shift
    f_list_dirs $@
    exit
elif [[ $1 == "list-files" ]]; then
    shift
    f_list_files $@
    exit
elif [[ $1 == "run" ]]; then
    shift
    f_run $@
    exit
elif [[ $1 == "run-all" ]]; then
    shift
    f_run_all $@
    exit
elif [[ $1 == "help" ]]; then
    shift
    f_help
    exit
else
    log_error "[ERROR] Command not supported!"
    f_help
    exit 1
fi
