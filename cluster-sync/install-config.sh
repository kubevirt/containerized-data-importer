#!/usr/bin/env bash

#Listed here a variables to be set by each provider provider  per installation technique
CDI_INSTALL_OPERATOR="install-operator" #install CDI via operator manifests
CDI_INSTALL_OLM="install-olm"           #install CDI via OLM CSV manifest
CDI_OLM_MANIFESTS_CATALOG_SRC=""        #location of CatalogSource manifest for OLM installation
CDI_OLM_MANIFESTS_SUBSCRIPTION=""       #location of Subscription manifest for OLM installation
CDI_INSTALL=${CDI_INSTALL:-${CDI_INSTALL_OPERATOR}} #installation technique set to operator manifests by default
CDI_INSTALL_TIMEOUT=${CDI_INSTALL_TIMEOUT:-120}     #timeout for installation sequence
