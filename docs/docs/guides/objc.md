---
title: Objective-C on Miren
description: Deploy an Objective-C program as a web service on Miren with a Dockerfile.miren using GNUstep.
keywords: [objective-c, objc, gnustep, foundation, dockerfile, deploy]
---

import CliCommand from '@site/src/components/CliCommand';

# Objective-C on Miren

Objective-C isn't auto-detected, so you deploy it with a `Dockerfile.miren` that
compiles your program with [GNUstep](https://www.gnustep.org) (the Foundation framework
on Linux) and runs the binary. This guide uses Foundation for the response and POSIX
sockets to serve it.

:::tip[Let your agent do this]
Ask your AI coding agent to "set up this Objective-C app on Miren" after installing the
[Miren agent skills](/agent-skills). It adds the `Dockerfile.miren` and GNUstep build,
and deploys — using this page as its reference.
:::

## Do you need a Dockerfile?

Yes. Add a `Dockerfile.miren` to your project root. Miren builds from it instead of
guessing the stack — see [Using Dockerfile.miren](/languages#using-dockerfilemiren).

## Bind to the injected port

Miren injects `PORT` and routes traffic to it. Read `PORT`, build the response with
Foundation, and serve it on `0.0.0.0`:

```objc
#import <Foundation/Foundation.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>
#include <arpa/inet.h>

int main(void) {
    NSAutoreleasePool *pool = [[NSAutoreleasePool alloc] init];

    const char *portEnv = getenv("PORT");
    int port = portEnv ? atoi(portEnv) : 8080;

    NSString *body = @"Hello from Objective-C on Miren!\n";
    NSString *response = [NSString stringWithFormat:
        @"HTTP/1.1 200 OK\r\nContent-Type: text/plain\r\nConnection: close\r\n\r\n%@", body];
    const char *resp = [response UTF8String];

    int sock = socket(AF_INET, SOCK_STREAM, 0);
    int opt = 1;
    setsockopt(sock, SOL_SOCKET, SO_REUSEADDR, &opt, sizeof(opt));

    struct sockaddr_in addr;
    memset(&addr, 0, sizeof(addr));
    addr.sin_family = AF_INET;
    addr.sin_addr.s_addr = INADDR_ANY;
    addr.sin_port = htons(port);

    if (bind(sock, (struct sockaddr *)&addr, sizeof(addr)) < 0) { perror("bind"); return 1; }
    listen(sock, 16);
    NSLog(@"listening on 0.0.0.0:%d", port);

    for (;;) {
        int client = accept(sock, NULL, NULL);
        if (client < 0) continue;
        char buf[1024];
        (void)read(client, buf, sizeof(buf));
        write(client, resp, strlen(resp));
        close(client);
    }

    [pool release];
    return 0;
}
```

:::warning[Use `NSAutoreleasePool`, not `@autoreleasepool`]
GNUstep on Linux compiles with GCC's Objective-C runtime, which is the classic runtime —
it doesn't support the ObjC 2.0 `@autoreleasepool { … }` syntax and fails with
`stray '@' in program`. Use `[[NSAutoreleasePool alloc] init]` / `[pool release]` instead.
:::

## The Dockerfile

Build with GNUstep's makefile system (install `make` — `gnustep-make` provides only the
makefiles, not GNU make itself):

```dockerfile
# ----- Build stage -----
FROM debian:12 AS build
RUN apt-get update -y \
    && apt-get install -y make gobjc gnustep-make gnustep-base-runtime libgnustep-base-dev \
    && rm -rf /var/lib/apt/lists/*
WORKDIR /app
COPY main.m GNUmakefile ./
RUN . /usr/share/GNUstep/Makefiles/GNUstep.sh && make
RUN cp ./obj/app /app/app.bin

# ----- Runtime stage -----
FROM debian:12-slim
RUN apt-get update -y && apt-get install -y gnustep-base-runtime && rm -rf /var/lib/apt/lists/*
COPY --from=build /app/app.bin /usr/local/bin/app
EXPOSE 8080
CMD ["app"]
```

The `GNUmakefile`:

```makefile
include $(GNUSTEP_MAKEFILES)/common.make

TOOL_NAME = app
app_OBJC_FILES = main.m

include $(GNUSTEP_MAKEFILES)/tool.make
```

### .dockerignore

```text
.git
```

## Deploy

Create `.miren/app.toml` naming your app and deploy from your project root:

```toml
name = "objc-bench"
```

<CliCommand context="client">
```miren
miren deploy
```
</CliCommand>

:::note[The Procfile is required]
Even with a `Dockerfile.miren`, Miren needs at least one service defined:

```procfile
web: /usr/local/bin/app
```
:::

## Agent quick reference

- **Detection:** none — requires `Dockerfile.miren`
- **Build:** GNUstep makefiles — install `make gobjc gnustep-make libgnustep-base-dev`; `make` outputs `obj/<tool>`
- **Classic runtime:** use `NSAutoreleasePool` (not `@autoreleasepool`); GCC's ObjC runtime is the classic one
- **Runtime lib:** `gnustep-base-runtime` on `debian-slim`
- **Service is required:** `Procfile` `web: /usr/local/bin/app` — the image `CMD` is not used
- **Port:** `getenv("PORT")`; bind `INADDR_ANY` (`0.0.0.0`)

## Next steps

- [C on Miren](/guides/c) — the C guide (Objective-C is a superset of C)
- [Using Dockerfile.miren](/languages#using-dockerfilemiren) — how custom builds work
- [Deployment](/deployment) — how deploys build and activate
