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
#include <fcntl.h>
#include <stdio.h>
#include <string.h>
#include <sys/stat.h>
#include <sys/types.h>
#include <unistd.h>

#define THREAD_MODEL NBDKIT_THREAD_MODEL_SERIALIZE_ALL_REQUESTS

int arg_count = 0;
int expected_arg_count = 7;

void fakevddk_close(void *handle) {
    close(*((int *) handle));
}

int fakevddk_config(const char *key, const char *value) {
    arg_count++;
    if (strcmp(key, "snapshot") == 0) {
        expected_arg_count = 9; // Expect one for 'snapshot' and one for 'transports'
    }
    if (strcmp(key, "config") == 0) {
        expected_arg_count = 8;
        nbdkit_debug("Extra config option set to: %s\n", value);

        FILE *f  = fopen(value, "r");
        if (f == NULL) {
            nbdkit_error("Failed to open VDDK extra configuration file %s!\n", value);
            return -1;
        }
        char extras[50];
        if (fgets(extras, 50, f) == NULL) { // Expect only one line of test data
            nbdkit_error("Failed to read VDDK extra configuration file %s! Error was: %s", value, strerror(errno));
            return -1;
        }
        if (strcmp(extras, "VixDiskLib.nfcAio.Session.BufSizeIn64KB=16") != 0) { // Must match datavolume_test
            nbdkit_error("Unexpected content in VDDK extra configuration file %s: %s\n", value, extras);
            return -1;
        }
        fclose(f);
    }
    return 0;
}

int fakevddk_config_complete(void) {
    nbdkit_debug("VMware VixDiskLib (1.2.3) Release build-12345");
    if (arg_count == expected_arg_count) {
        return 0;
    } else {
        nbdkit_error("Expected %d arguments to fake VDDK test plugin, but got %d!\n", expected_arg_count, arg_count);
        return -1;
    }
}

void *fakevddk_open(int readonly) {
    static int fd;
    fd = open("/opt/testing/nbdtest.img", O_RDONLY);
    if (fd == -1) {
        nbdkit_error("Failed to open /opt/testing/nbdtest.img: %m");
        return NULL;
    }
    nbdkit_debug("DISKLIB-LINK  : Opened 'vpxa-nfcssl://[iSCSI_Datastore] test/test.vmdk@esx.test:902' (0xa): custom, 50331648 sectors / 24 GB.");
    return (void *) &fd;
}

// Avoid requiring glibc 2.33 by using the older fstat
extern int __fxstat(int, int, struct stat *);
#define fstat(fd, buf) __fxstat(1, fd, buf)
int64_t fakevddk_get_size(void *handle) {
    struct stat info;
    fstat(*((int *) handle), &info);
    return info.st_size;
}

int fakevddk_pread(void *handle, void *buf, uint32_t count, uint64_t offset, uint32_t flags) {
    uint64_t read_offset = offset;
    uint32_t read_count = count;
    void *read_buffer = buf;

    while (read_count > 0) {
        ssize_t r = pread(*((int *)handle), read_buffer, read_count, read_offset);
        if (r == 0) {
            nbdkit_error("End-of-file from pread!");
            return -1;
        } else if (r == -1) {
            nbdkit_error("Error from pread: %m");
            return -1;
        } else {
            read_buffer += r;
            read_offset += r;
            read_count -= r;
        }
    }

    return 0;
}

struct nbdkit_plugin fakevddk = {
    .name            = "vddk",
    .longname        = "Simulated VDDK plugin for CDI testing",
    .version         = "N/A",
    .close           = fakevddk_close,
    .config          = fakevddk_config,
    .config_complete = fakevddk_config_complete,
    .open            = fakevddk_open,
    .get_size        = fakevddk_get_size,
    .pread           = fakevddk_pread,
};

NBDKIT_REGISTER_PLUGIN(fakevddk)
