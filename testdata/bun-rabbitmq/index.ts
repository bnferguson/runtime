import * as amqp from "amqplib";

const url = process.env.RABBITMQ_URL!;
const testQueue = "bun-test-queue";

async function publishAndConsume(): Promise<string> {
  const conn = await amqp.connect(url);
  const ch = await conn.createChannel();

  await ch.assertQueue(testQueue, { durable: false });

  const message = `hello-${Date.now()}`;
  ch.sendToQueue(testQueue, Buffer.from(message));

  // Consume the message we just published.
  const result = await new Promise<string>((resolve, reject) => {
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

  await ch.close();
  await conn.close();
  return result;
}

async function healthCheck(): Promise<boolean> {
  const conn = await amqp.connect(url);
  const ch = await conn.createChannel();
  await ch.assertQueue("health-check", { durable: false });
  await ch.deleteQueue("health-check");
  await ch.close();
  await conn.close();
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
