import http from "k6/http";
import { check } from "k6";

export const options = {
  scenarios: {
    redirect: {
      executor: "constant-arrival-rate",
      rate: 15432,
      timeUnit: "1s",
      duration: "1m",
      preAllocatedVUs: 250,
      maxVUs: 800,
    },
  },
  thresholds: {
    http_req_failed: [{ threshold: "rate<0.001", abortOnFail: true }],
    http_req_duration: [{ threshold: "p(99)<100", abortOnFail: true }],
  },
};

export default function () {
  const base = __ENV.BASE_URL;
  const code = __ENV.SHORT_CODE;
  if (!base || !code) {
    throw new Error("BASE_URL and SHORT_CODE required");
  }
  const res = http.get(`${base.replace(/\/$/, "")}/${code}`, { redirects: 0 });
  check(res, { "302": (r) => r.status === 302 });
}
