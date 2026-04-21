import { connect, Socket } from "net";

const url = new URL(process.env.VALKEY_URL!);
const host = url.hostname;
const port = Number(url.port || 6379);
const password = decodeURIComponent(url.password || "");

function encodeCommand(args: string[]): string {
  let out = `*${args.length}\r\n`;
  for (const a of args) {
    out += `$${Buffer.byteLength(a)}\r\n${a}\r\n`;
  }
  return out;
}

// Parse a single RESP reply starting at offset `i` in `buf`.
// Returns [value, newOffset] or throws if more data is needed or on protocol error.
// Value types: string (simple or bulk), number (integer), null (nil bulk), Error (error reply).
function parseReply(buf: string, i: number): [string | number | null | Error, number] {
  if (i >= buf.length) throw new Error("incomplete");
  const type = buf[i];
  const lineEnd = buf.indexOf("\r\n", i + 1);
  if (lineEnd < 0) throw new Error("incomplete");
  const head = buf.slice(i + 1, lineEnd);
  if (type === "+") return [head, lineEnd + 2];
  if (type === "-") return [new Error(head), lineEnd + 2];
  if (type === ":") return [Number(head), lineEnd + 2];
  if (type === "$") {
    const len = Number(head);
    if (len === -1) return [null, lineEnd + 2];
    const dataEnd = lineEnd + 2 + len;
    if (buf.length < dataEnd + 2) throw new Error("incomplete");
    return [buf.slice(lineEnd + 2, dataEnd), dataEnd + 2];
  }
  throw new Error(`unsupported reply type: ${type}`);
}

class ValkeyClient {
  private sock: Socket;
  private buf = "";
  private pending: Array<(v: string | number | null | Error) => void> = [];

  constructor(sock: Socket) {
    this.sock = sock;
    sock.on("data", (chunk) => {
      this.buf += chunk.toString("binary");
      this.drain();
    });
  }

  private drain() {
    while (this.pending.length > 0) {
      try {
        const [value, next] = parseReply(this.buf, 0);
        this.buf = this.buf.slice(next);
        this.pending.shift()!(value);
      } catch {
        return; // wait for more bytes
      }
    }
  }

  send(args: string[]): Promise<string | number | null> {
    return new Promise((resolve, reject) => {
      this.pending.push((v) => (v instanceof Error ? reject(v) : resolve(v)));
      this.sock.write(encodeCommand(args));
    });
  }

  close() {
    this.sock.end();
  }
}

function dial(): Promise<ValkeyClient> {
  return new Promise((resolve, reject) => {
    const sock = connect(port, host);
    sock.setTimeout(5000, () => {
      sock.destroy();
      reject(new Error("valkey connect timeout"));
    });
    sock.once("error", reject);
    sock.once("connect", async () => {
      const client = new ValkeyClient(sock);
      try {
        if (password) await client.send(["AUTH", password]);
        resolve(client);
      } catch (e) {
        sock.destroy();
        reject(e);
      }
    });
  });
}

const server = Bun.serve({
  port: process.env.PORT || 3000,

  async fetch(req) {
    const u = new URL(req.url);

    if (u.pathname === "/") {
      const client = await dial();
      try {
        const count = await client.send(["INCR", "visits"]);
        return new Response(`Hello! Visit count: ${count}\n`);
      } finally {
        client.close();
      }
    }

    if (u.pathname === "/health") {
      try {
        const client = await dial();
        try {
          await client.send(["SET", "healthcheck", "ok"]);
          const val = await client.send(["GET", "healthcheck"]);
          if (val === "ok") return new Response("ok\n");
          return new Response(`valkey read mismatch: ${val}\n`, { status: 503 });
        } finally {
          client.close();
        }
      } catch (e) {
        return new Response(`valkey unavailable: ${e}\n`, { status: 503 });
      }
    }

    return new Response("not found\n", { status: 404 });
  },
});

console.log(`Listening on http://localhost:${server.port}`);
