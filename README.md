# oci

## Name

_oci_ - pull OCI artifacts from repositories

## Description

_oci_ pulls an OCI artifact into the site using the [oras Go SDK](https://pkg.go.dev/oras.land/oras-go/v2@v2.5.0). This makes it possible to deploy your zones with a simple oci pull.

The _oci_ plugin starts a service routine that runs during the lifetime of the server. When the
service starts, it pulls the artifacts from the repository.

If a pull fails, the service will retry up to three time. Each request will additionally try 3 times if certain response are returned by the repository.

NOTE: This plugin makes use of oras' retry client and will attempt additional retries for each attempt. See [oras/registry/remote/retry](https://pkg.go.dev/oras.land/oras-go/v2@v2.5.0/registry/remote/retry) for the default retry policy.

If the pull was not successful by then, it won't try again until the next interval.

This plugin is inspired by MiekG's `git` plugin and borrows some syntax and logic.

## Syntax

```txt
oci REPO [PATH]
```

- **REPO** is the URL to the repository

- **PATH** is the path, relative to site root, to pull the artifacts into; default is site root

This simplified syntax pulls the `latest` tag every 3600 seconds (1 hour) and only works for public
repositories.

For more control or to use a private repository, use the following syntax:

```
oci [REPO PATH] {
  repo              REPO
  path              PATH
  username      USERNAME
  password      PASSWORD
  interval         INTERVAL
}
```

- **REPO** is the URL to the repository; only HTTP/S URLs (http(s):// must be ommitted) are supported.

- **PATH** is the path to pull the artifacts into; default is site root (if set). It can be
  absolute or relative (to site root). See the _root_ plugin.

- **INTERVAl** is the number of seconds between pulls; default is 3600 (1 hour), minimum 5.

- **USERNAME** is the username to log into the remote repository.

- **PASSWORD** is the password to log into the remote repository`

- **IMSECURE** if set to "true" connects to the repository with plain HTTP

## Examples

Public repository pulled into site root every hour:

```corefile
example.org {
    root /etc/zones
    oci registry-1.docker.io/ganawaj/demo:0.0.2
}
```

Private repository pulled into "example.org" directory or /etc/zones/example.org

```corefile
example.org {
    root /etc/zones
    oci registry-1.docker.io/ganawaj/demo:0.0.2 {
      path example.org
      username ganawaj
      password dckr_pat_THISISANEXAMPLE
      interval 10
    }
}
```

Local private repository with no authentication using plain HTTP. Tag `latest` is assummed.

```corefile
example.org {
    root /etc/zones
    oci localhost:5000/ganawaj/demo {
      path example.org
      insecure true
    }
}
```

Full example using _file_ plugin assuming `demo` pulls a `db.example.org` artifact

```corefile
example.org {
    root /etc/zones

    oci registry-1.docker.io/ganawaj/demo:0.0.2 {
      path example.org
      username ganawaj
      password dckr_pat_THISISANEXAMPLE
      interval 10
    }

    file example.org/db.example.org
}
```

## Also See

The _root_ plugin for setting the root.

The _git_ plugin for inspiration for this plugin.

The _auto_ or _file_ plugin for reading zone files from disk.
