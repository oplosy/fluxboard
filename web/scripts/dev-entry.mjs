import { spawn } from 'node:child_process';

const child = spawn('pnpm', ['dev'], { stdio: 'inherit' });
// Next dev compiles a route on its first request (1–2 s each in the container),
// so warm the app's main screens up front. Dynamic routes compile once per
// pattern; the demo seed's `acme`/`WEB` just gives them concrete URLs.
const project = '/app/acme/projects/WEB';
const routes = [
  '/', '/login', '/app', '/account/profile', '/account/notifications', '/account/sessions',
  '/app/acme', '/app/acme/projects', '/app/acme/my-tasks', '/app/acme/dashboard', '/app/acme/goals',
  project, `${project}/list`, `${project}/sprints`, `${project}/epics`, `${project}/releases`,
  `${project}/workload`, `${project}/analytics`, `${project}/docs`, `${project}/calendar`, `${project}/timeline`,
  `${project}/settings`, `${project}/tasks/1`,
];

for (let attempt = 0; attempt < 60; attempt += 1) {
  try {
    const response = await fetch('http://127.0.0.1:3000/');
    if (response.ok) break;
  } catch {
    // Next is still starting.
  }
  await new Promise((resolve) => setTimeout(resolve, 500));
}

await Promise.all(
  routes.map(async (route) => {
    try {
      await fetch(`http://127.0.0.1:3000${route}`);
    } catch {
      // A failed warm-up must not hide the dev server process.
    }
  }),
);

const forward = (signal) => child.kill(signal);
process.on('SIGTERM', () => forward('SIGTERM'));
process.on('SIGINT', () => forward('SIGINT'));

child.on('exit', (code, signal) => {
  process.exit(code ?? (signal ? 1 : 0));
});
