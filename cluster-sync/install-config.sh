#!/usr/bin/env bash

#Listed here a variables to be set by each provider provider  per installation technique
CDI_INSTALL="install-operator" #install CDI via operator manifests
CDI_INSTALL_TIMEOUT=${CDI_INSTALL_TIMEOUT:-120}     #timeout for installation sequence
