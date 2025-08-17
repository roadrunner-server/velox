import http from "k6/http";
import { check } from "k6";
import { Rate } from "k6/metrics";
import { uuidv4 } from "https://jslib.k6.io/k6-utils/1.4.0/index.js";

// Platform configurations: OS + Architecture combinations
const platforms = [
  { os: "linux", arch: "amd64" }, // linux + amd64
  { os: "linux", arch: "arm64" }, // linux + arm64
  { os: "windows", arch: "amd64" }, // windows + amd64
  { os: "windows", arch: "arm64" }, // windows + arm64
  { os: "darwin", arch: "amd64" }, // darwin + amd64
  { os: "darwin", arch: "arm64" }, // darwin + arm64
];

// Custom metrics
export const errorRate = new Rate("errors");

// Test configuration
export const options = {
  scenarios: {
    // Constant load test
    constant_load: {
      executor: "constant-vus",
      vus: 50, // 50 virtual users
      duration: "60s", // for 30 seconds
    },
    // Ramping load test (uncomment to use instead)
    // ramping_load: {
    //   executor: 'ramping-vus',
    //   startVUs: 1,
    //   stages: [
    //     { duration: '10s', target: 5 },   // Ramp up to 5 users
    //     { duration: '20s', target: 10 },  // Stay at 10 users
    //     { duration: '10s', target: 20 },  // Ramp up to 20 users
    //     { duration: '20s', target: 20 },  // Stay at 20 users
    //     { duration: '10s', target: 0 },   // Ramp down
    //   ],
    // },
  },
  thresholds: {
    http_req_duration: ["p(95)<30000"], // 95% of requests must complete below 30s
    http_req_failed: ["rate<0.1"], // Error rate must be below 10%
    errors: ["rate<0.1"], // Custom error rate below 10%
  },
};

// Request payload
const payload = {
  request_id: "",
  force_rebuild: true,
  target_platform: {
    os: "linux",
    arch: "amd64",
  },
  rr_version: "v2025.1.2",
  plugins: [
    {
      module_name: "github.com/roadrunner-server/app-logger/v5",
      tag: "v5.1.8",
    },
    {
      module_name: "github.com/roadrunner-server/logger/v5",
      tag: "v5.1.8",
    },
    {
      module_name: "github.com/roadrunner-server/lock/v5",
      tag: "v5.1.8",
    },
    {
      module_name: "github.com/roadrunner-server/rpc/v5",
      tag: "v5.1.8",
    },
    {
      module_name: "github.com/roadrunner-server/centrifuge/v5",
      tag: "v5.1.8",
    },
    {
      module_name: "github.com/temporalio/roadrunner-temporal/v5",
      tag: "v5.7.0",
    },
    {
      module_name: "github.com/roadrunner-server/metrics/v5",
      tag: "v5.1.8",
    },
    {
      module_name: "github.com/roadrunner-server/otel/v5",
      tag: "v5.1.8",
    },
    {
      module_name: "github.com/roadrunner-server/http/v5",
      tag: "v5.2.7",
    },
    {
      module_name: "github.com/roadrunner-server/gzip/v5",
      tag: "v5.1.8",
    },
    {
      module_name: "github.com/roadrunner-server/prometheus/v5",
      tag: "v5.1.7",
    },
    {
      module_name: "github.com/roadrunner-server/headers/v5",
      tag: "v5.1.8",
    },
    {
      module_name: "github.com/roadrunner-server/static/v5",
      tag: "v5.1.6",
    },
    {
      module_name: "github.com/roadrunner-server/proxy_ip_parser/v5",
      tag: "v5.1.8",
    },
    {
      module_name: "github.com/roadrunner-server/send/v5",
      tag: "v5.1.5",
    },
    {
      module_name: "github.com/roadrunner-server/server/v5",
      tag: "v5.2.9",
    },
    {
      module_name: "github.com/roadrunner-server/service/v5",
      tag: "v5.1.8",
    },
    {
      module_name: "github.com/roadrunner-server/jobs/v5",
      tag: "v5.1.8",
    },
    {
      module_name: "github.com/roadrunner-server/amqp/v5",
      tag: "v5.2.2",
    },
    {
      module_name: "github.com/roadrunner-server/sqs/v5",
      tag: "v5.1.8",
    },
    {
      module_name: "github.com/roadrunner-server/beanstalk/v5",
      tag: "v5.1.8",
    },
    {
      module_name: "github.com/roadrunner-server/nats/v5",
      tag: "v5.1.8",
    },
    {
      module_name: "github.com/roadrunner-server/kafka/v5",
      tag: "v5.2.4",
    },
    {
      module_name: "github.com/roadrunner-server/google-pub-sub/v5",
      tag: "v5.1.8",
    },
    {
      module_name: "github.com/roadrunner-server/kv/v5",
      tag: "v5.2.8",
    },
    {
      module_name: "github.com/roadrunner-server/boltdb/v5",
      tag: "v5.1.8",
    },
    {
      module_name: "github.com/roadrunner-server/memory/v5",
      tag: "v5.2.8",
    },
    {
      module_name: "github.com/roadrunner-server/redis/v5",
      tag: "v5.1.9",
    },
    {
      module_name: "github.com/roadrunner-server/memcached/v5",
      tag: "v5.1.8",
    },
    {
      module_name: "github.com/roadrunner-server/fileserver/v5",
      tag: "v5.1.8",
    },
    {
      module_name: "github.com/roadrunner-server/grpc/v5",
      tag: "v5.2.2",
    },
    {
      module_name: "github.com/roadrunner-server/status/v5",
      tag: "v5.1.8",
    },
    {
      module_name: "github.com/roadrunner-server/tcp/v5",
      tag: "v5.1.8",
    },
  ],
};

// Request headers
const headers = {
  "Content-Type": "application/json",
  Accept: "application/json",
};

// Main test function
export default function () {
  // Randomly select a platform for this request
  const randomPlatform =
    platforms[Math.floor(Math.random() * platforms.length)];

  // Generate unique request_id using UUID
  const uniquePayload = {
    ...payload,
    request_id: uuidv4(),
    target_platform: randomPlatform,
  };

  const response = http.post(
    "http://127.0.0.1:9000/api.service.v1.BuildService/Build",
    JSON.stringify(uniquePayload),
    {
      headers: headers,
      timeout: "120s", // Allow up to 60 seconds for build operations
    },
  );

  // Check response
  const isSuccess = check(response, {
    "status is 200": (r) => r.status === 200,
    "response time < 30000ms": (r) => r.timings.duration < 30000,
    "response has body": (r) => r.body && r.body.length > 0,
    "content-type is application/json": (r) =>
      r.headers["Content-Type"] &&
      r.headers["Content-Type"].includes("application/json"),
  });

  // Track errors
  errorRate.add(!isSuccess);

  // Log response details for debugging (only for failed requests)
  if (!isSuccess) {
    console.log(`Request failed:
      Status: ${response.status}
      Duration: ${response.timings.duration}ms
      Body: ${response.body.substring(0, 200)}...
      Request ID: ${uniquePayload.request_id}
      Platform: ${randomPlatform.os} + ${randomPlatform.arch}
    `);
  }
}

// Setup function (runs once per VU at the beginning)
export function setup() {
  console.log("Starting velox build service benchmark...");
  console.log(
    "Target: http://127.0.0.1:9000/api.service.v1.BuildService/Build",
  );
  console.log("Plugins count:", payload.plugins.length);
  console.log(
    "Available platforms:",
    platforms.map((p) => `${p.os} + ${p.arch}`).join(", "),
  );
  console.log("Each request will randomly select a platform combination");
  return {};
}

// Teardown function (runs once at the end)
export function teardown(data) {
  console.log("Benchmark completed!");
}
