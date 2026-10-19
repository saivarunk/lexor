import type { components } from "../types/api.generated";
export type Preferences = components["schemas"]["Preferences"];
export type Problem = components["schemas"]["Problem"];
export type ProblemView = components["schemas"]["ProblemView"];
export type Execution = components["schemas"]["ExecutionResult"];
export type Draft = components["schemas"]["Draft"];
export type Stats = components["schemas"]["Stats"];
export type SystemStatus = components["schemas"]["SystemStatus"];
export type ImportPackage = components["schemas"]["Package"];
export type SyncPreview = components["schemas"]["Preview"];
let token: Promise<string> | undefined;
async function csrf(): Promise<string> {
  token ??= fetch("/api/v1/session")
    .then(async (r) => {
      if (!r.ok) throw new Error("Cannot connect to Lexor");
      return (await r.json()).csrfToken as string;
    })
    .catch((e) => {
      token = undefined;
      throw e;
    });
  return token;
}
export async function api<T>(path: string, init: RequestInit = {}): Promise<T> {
  const headers = new Headers(init.headers);
  if (init.method && !["GET", "HEAD"].includes(init.method)) {
    headers.set("Content-Type", "application/json");
    headers.set("X-Lexor-CSRF", await csrf());
  }
  const response = await fetch(`/api/v1${path}`, { ...init, headers });
  if (!response.ok) {
    const error = await response.json().catch(() => null);
    if (response.status === 403) token = undefined;
    throw new Error(
      error?.error?.message ?? `Request failed (${response.status})`,
    );
  }
  if (response.status === 204) return undefined as T;
  return response.json();
}
export const post = <T>(path: string, body: unknown = {}) =>
  api<T>(path, { method: "POST", body: JSON.stringify(body) });
export function stripProblem(p: ProblemView): Problem {
  return {
    schemaVersion: p.schemaVersion,
    id: p.id,
    slug: p.slug,
    title: p.title,
    description: p.description,
    difficulty: p.difficulty,
    organization: p.organization,
    constraints: p.constraints,
    examples: p.examples,
    languages: p.languages,
    tests: p.tests,
    comparison: p.comparison,
    metadata: p.metadata,
  };
}
export function formatDate(value?: string) {
  return value
    ? new Date(value).toLocaleString(undefined, {
        month: "short",
        day: "numeric",
        hour: "2-digit",
        minute: "2-digit",
      })
    : "—";
}
