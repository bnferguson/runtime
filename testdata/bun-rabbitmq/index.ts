import * as amqp from "amqplib";

const url = process.env.RABBITMQ_URL!;
const testQueue = "bun-test-queue";

async function publishAndConsume(): Promise<string> {
  const conn = await amqp.connect(url);
  // Connection-level errors (e.g. the broker closing the connection) surface
  // as async 'error' events, not as rejections of the awaited calls below.
  // Without a handler they become uncaught exceptions and crash the process.
  conn.on("error", () => {});
  const ch = await conn.createChannel();
  ch.on("error", () => {});

  try {
    // RabbitMQ 4.3+ denies transient non-exclusive queues by default (the
    // transient_nonexcl_queues deprecated feature went denied_by_default in
    // 4.3.0), so this queue must be durable.
    await ch.assertQueue(testQueue, { durable: true });

    const message = `hello-${Date.now()}`;
    ch.sendToQueue(testQueue, Buffer.from(message));

    // Consume the message we just published.
    return await new Promise<string>((resolve, reject) => {
      const timeout = setTimeout(() => reject(new Error("consume timeout")), 5000);
      ch.consume(
        testQueue,
        (msg) => {
          if (msg) {
            clearTimeout(timeout);
            ch.ack(msg);
            resolve(msg.content.toString());
          }
        },
        { noAck: false }
      );
    });
  } finally {
    // Always release the channel/connection, even on the error or timeout
    // paths, so repeated failures can't leak connections against broker limits.
    await ch.close().catch(() => {});
    await conn.close().catch(() => {});
  }
}

async function healthCheck(): Promise<boolean> {
  const conn = await amqp.connect(url);
  conn.on("error", () => {});
  const ch = await conn.createChannel();
  ch.on("error", () => {});
  try {
    // Exclusive queues are connection-scoped and auto-delete on close, so they
    // sidestep the transient_nonexcl_queues restriction without needing
    // durability or an explicit delete. Use a server-generated name (empty
    // string) so concurrent health checks can't collide on a locked name.
    await ch.assertQueue("", { exclusive: true });
  } finally {
    await ch.close().catch(() => {});
    await conn.close().catch(() => {});
  }
  return true;
}

const server = Bun.serve({
  port: process.env.PORT || 3000,

  async fetch(req) {
    const u = new URL(req.url);

    if (u.pathname === "/") {
      try {
        const msg = await publishAndConsume();
        return new Response(`RabbitMQ round-trip OK: ${msg}\n`);
      } catch (e) {
        return new Response(`RabbitMQ error: ${e}\n`, { status: 503 });
      }
    }

    if (u.pathname === "/health") {
      try {
        await healthCheck();
        return new Response("ok\n");
      } catch (e) {
        return new Response(`rabbitmq unavailable: ${e}\n`, { status: 503 });
      }
    }

    return new Response("not found\n", { status: 404 });
  },
});

console.log(`Listening on http://localhost:${server.port}`);
