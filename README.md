# NoCloud Metadata Server

A simple service for dynamically configuring Cloud-Init VMs using NoCloud, such
as in edge compute environments and homelabs.

## Config file structure

The server's configuration is loaded from a `config.yaml` file. It defaults to
checking the current directory. Alternate paths can be specified using the
`--config=path/to/config.yaml` flag.

The configuration is comprised of two main parts:

1.  Server configs which defines basic information such as the regex patterns
    to trigger on, and the hostname to set.
1.  User data templates, which define any number of
    [user data modules](https://cloudinit.readthedocs.io/en/latest/reference/modules.html)
    to be provisioned.

A user data template may be referenced by multiple server configs via the
`userDataTemplate` field. Each server config also has a `replacements` map that
allows multiple similar server configs to reuse a single user data template,
while customizing some fields by merging the map with the template.

### Example

This example has a common base config which updates the system's packages
on first boot, disables SSH password authentication, and then includes 
dev OR prod SSH keys depending on the requested URL.

```yaml
userDataTemplates:
  basic:
    package_upgrade: true
    ssh_pwauth: false

serverConfigs:
- name: "basic-dev"
  matchPatterns:
  - "dev"
  instanceConfig:
    hostname: "basic-dev"
    enableInstanceIDSuffix: true
    enableHostnameSuffix: true
  userDataTemplate: "basic"
  replacements:
    ssh_authorized_keys:
    - ssh-rsa dev-management-ssh-key...

- name: "basic-prod"
  matchPatterns:
  - "prod"
  instanceConfig:
    hostname: "basic-prod"
    enableInstanceIDSuffix: true
    enableHostnameSuffix: true
  userDataTemplate: "basic"
  replacements:
    ssh_authorized_keys:
    - ssh-rsa prod-management-ssh-key...
```

```sh
$ curl localhost:8000/dev/meta-data
instance-id: i-dev-4a467d03
local-hostname: basic-dev-4a467d03
hostname: basic-dev-4a467d03

$ curl localhost:8000/dev/user-data
package_upgrade: true
ssh_authorized_keys:
  - ssh-rsa dev-management-ssh-key...
ssh_pwauth: false

$ curl localhost:8000/prod/meta-data
instance-id: i-prod-0138fed4
local-hostname: basic-prod-0138fed4
hostname: basic-prod-0138fed4

$ curl localhost:8000/prod/user-data
package_upgrade: true
ssh_authorized_keys:
  - ssh-rsa prod-management-ssh-key...
ssh_pwauth: false
```

## Integration with QEMU/KVM

When creating QEMU VMs, you configure the SMBIOS serial such that Cloud-Init
will retrieve the configuration from the metadata server after networking
has been initialized.

Continuing the config example above, the options below would configure
Cloud-Init to use the NoCloud data store, and to fetch the `dev` configuration
from the metadata server running at `10.10.10.10:8000`.

```
-smbios type=1,serial=ds=nocloud-net;s=http://10.10.10.10:8000/dev/
```

See the [NoCloud docs](https://cloudinit.readthedocs.io/en/latest/reference/datasources/nocloud.html)
and this helpful [chart](https://gist.github.com/smoser/290f74c256c89cb3f3bd434a27b9f64c)
mapping DMI/SMBIOS options for more information on this subject.
