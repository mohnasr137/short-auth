import http from 'k6/http';
import { check } from 'k6';

// Production k6 Benchmark Scenario Configuration
export const options = {
  scenarios: {
    // Stage 1: Warm-up (1,000 RPS for 5s)
    warmup: {
      executor: 'constant-arrival-rate',
      rate: 1000,
      timeUnit: '1s',
      duration: '5s',
      preAllocatedVUs: 50,
      maxVUs: 200,
      startTime: '0s',
    },
    // Stage 2: Target 10,000 RPS Stress Test
    target_10k_benchmark: {
      executor: 'ramping-arrival-rate',
      startRate: 2000,
      timeUnit: '1s',
      preAllocatedVUs: 100,
      maxVUs: 1000,
      startTime: '6s',
      stages: [
        { target: 5000, duration: '5s' },   // Ramp up to 5,000 RPS
        { target: 10000, duration: '15s' }, // Push to 10,000 RPS
        { target: 10000, duration: '15s' }, // Hold steady at 10,000 RPS
        { target: 0, duration: '3s' },      // Ramp down
      ],
    },
  },
  thresholds: {
    http_req_failed: ['rate<0.01'],    // Under 1% failures under extreme load
    http_req_duration: ['p(95)<25'],   // 95% of requests completed in < 25ms
    http_req_duration: ['p(99)<50'],   // 99% of requests completed in < 50ms
  },
};

// setup() runs once before the load test to authenticate and get a signed JWT
export function setup() {
  const loginUrl = 'http://localhost:3000/api/auth/login';
  const payload = JSON.stringify({
    username: 'bench_user',
    password: 'Password123!',
  });
  const params = {
    headers: { 'Content-Type': 'application/json' },
  };

  const res = http.post(loginUrl, payload, params);
  check(res, {
    'login successful': (r) => r.status === 200,
  });

  const body = JSON.parse(res.body);
  return { token: body.access_token };
}

// export default function runs for every virtual user request in the benchmark
export default function (data) {
  const url = 'http://localhost:3000/api/auth/verify';
  const params = {
    headers: {
      'Authorization': `Bearer ${data.token}`,
      'Connection': 'keep-alive',
    },
    tags: { name: 'TokenVerify' },
  };

  const res = http.get(url, params);
  check(res, {
    'status is 200': (r) => r.status === 200,
    'token is valid': (r) => {
      try {
        return JSON.parse(r.body).valid === true;
      } catch (e) {
        return false;
      }
    },
  });
}
