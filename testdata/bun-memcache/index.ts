import { connect } from "net";

const host = process.env.MEMCACHE_HOST;
const port = Number(process.env.MEMCACHE_PORT || 11211);

// Minimal memcached text protocol client using raw TCP.
function memcache(cmd: string): Promise<string> {
  return new Promise((resolve, reject) => {
    const sock = connect(port, host!, () => {
      sock.write(cmd + "\r\n");
    });
    let data = "";
    sock.on("data", (chunk) => {
      data += chunk.toString();
      // Memcached responses end with \r\n after the final line.
      // For get: "VALUE ... \r\n<data>\r\nEND\r\n"
      // For set/delete: "STORED\r\n" / "DELETED\r\n" / etc.
      if (
        data.endsWith("END\r\n") ||
        data.endsWith("STORED\r\n") ||
        data.endsWith("NOT_FOUND\r\n") ||
        data.endsWith("DELETED\r\n") ||
        data.endsWith("ERROR\r\n") ||
        data.endsWith("NOT_STORED\r\n")
      ) {
        sock.end();
        resolve(data.trim());
      }
    });
    sock.on("error", reject);
    sock.setTimeout(5000, () => {
      sock.destroy();
      reject(new Error("memcached timeout"));
    });
  });
}

async function mcSet(key: string, value: string): Promise<string> {
  return memcache(`set ${key} 0 0 ${Buffer.byteLength(value)}\r\n${value}`);
}

async function mcGet(key: string): Promise<string | null> {
  const resp = await memcache(`get ${key}`);
  if (resp === "END") return null;
  // "VALUE <key> <flags> <bytes>\r\n<data>\r\nEND"
  const lines = resp.split("\r\n");
  return lines.length >= 2 ? lines[1] : null;
}

const server = Bun.serve({
  port: process.env.PORT || 3000,

  async fetch(req) {
    const url = new URL(req.url);

    if (url.pathname === "/") {
      // Increment a visit counter in memcache
      const prev = await mcGet("visits");
      const count = (prev ? parseInt(prev, 10) : 0) + 1;
      await mcSet("visits", String(count));
      return new Response(`Hello! Visit count: ${count}\n`);
    }

    if (url.pathname === "/health") {
      try {
        await mcSet("healthcheck", "ok");
        const val = await mcGet("healthcheck");
        if (val === "ok") {
          return new Response("ok\n");
        }
        return new Response("memcache read mismatch\n", { status: 503 });
      } catch (e) {
        return new Response(`memcache unavailable: ${e}\n`, { status: 503 });
      }
    }

    return new Response("not found\n", { status: 404 });
  },
});

console.log(`Listening on http://localhost:${server.port}`);
