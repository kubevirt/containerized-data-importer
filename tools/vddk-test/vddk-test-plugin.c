//Licensed under the Apache License, Version 2.0 (the "License");
//you may not use this file except in compliance with the License.
//You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
//Unless required by applicable law or agreed to in writing, software
//distributed under the License is distributed on an "AS IS" BASIS,
//WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//See the License for the specific language governing permissions and
//limitations under the License.

#define NBDKIT_API_VERSION 2
#include <nbdkit-plugin.h>
#include <string.h>

#define THREAD_MODEL NBDKIT_THREAD_MODEL_SERIALIZE_ALL_REQUESTS
#define LIMIT 524288000

int fakevddk_config(const char *key, const char *value) {
    return 0;
}

int fakevddk_config_complete(void) {
    return 0;
}

void *fakevddk_open(int readonly) {
    return NBDKIT_HANDLE_NOT_NEEDED;
}

int64_t fakevddk_get_size(void *handle) {
    return (int64_t) LIMIT;
}

int fakevddk_pread(void *handle, void *buf, uint32_t count, uint64_t offset, uint32_t flags) {
    memset(buf, 0x55, count);
    return 0;
}

struct nbdkit_plugin fakevddk = {
    .name            = "vddk",
    .longname        = "Simulated VDDK plugin for CDI testing",
    .version         = "N/A",
    .config          = fakevddk_config,
    .config_complete = fakevddk_config_complete,
    .open            = fakevddk_open,
    .get_size        = fakevddk_get_size,
    .pread           = fakevddk_pread,
};

NBDKIT_REGISTER_PLUGIN(fakevddk)
