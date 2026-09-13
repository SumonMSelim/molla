import http from "k6/http";
import { check } from "k6";

export const options = {
  scenarios: {
    create: {
      executor: "constant-arrival-rate",
      rate: 154,
      timeUnit: "1s",
      duration: "1m",
      preAllocatedVUs: 40,
      maxVUs: 120,
    },
  },
  thresholds: {
    http_req_failed: [{ threshold: "rate<0.001", abortOnFail: true }],
    http_req_duration: [{ threshold: "p(99)<200", abortOnFail: true }],
  },
};

export default function () {
  const base = __ENV.BASE_URL;
  const key = __ENV.API_KEY;
  if (!base || !key) {
    throw new Error("BASE_URL and API_KEY required");
  }
  const res = http.post(
    `${base.replace(/\/$/, "")}/api/v1/links`,
    JSON.stringify({ long_url: "https://example.com/load" }),
    {
      headers: {
        "Content-Type": "application/json",
        "X-Api-Key": key,
      },
    },
  );
  check(res, { "201": (r) => r.status === 201 });
}
