import { test, expect, type Page } from "@playwright/test";
import { spawn, type ChildProcess } from "node:child_process";
import { mkdtemp, readFile, writeFile, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import path from "node:path";
let workspace: string, process: ChildProcess;
const root = path.resolve(import.meta.dirname, "../..");
async function start() {
  process = spawn(
    path.join(root, "bin/lexor"),
    ["--workspace", workspace, "serve", "--port", "8079"],
    {
      env: {
        ...globalThis.process.env,
        LEXOR_TIMEOUT_SECONDS: "12",
        LEXOR_PYRIGHT_COMMAND: path.join(
          root,
          "web/node_modules/.bin/pyright-langserver",
        ),
      },
      stdio: "ignore",
    },
  );
  for (let i = 0; i < 80; i++) {
    if (process.exitCode !== null || process.signalCode !== null)
      throw new Error("Lexor exited before startup; check port 8079");
    let ready = false;
    try {
      ready = (
        await fetch("http://127.0.0.1:8079/health", {
          signal: AbortSignal.timeout(1000),
        })
      ).ok;
    } catch {}
    if (ready) {
      const status = await (
        await fetch("http://127.0.0.1:8079/api/v1/system/status")
      ).json();
      if (status.workspace !== workspace)
        throw new Error(
          "Another test owns port 8079; run browser suites sequentially",
        );
      return;
    }
    await new Promise((r) => setTimeout(r, 100));
  }
  throw new Error("Lexor did not start");
}
async function stop() {
  if (!process || process.exitCode !== null || process.signalCode !== null)
    return;
  await new Promise<void>((resolve) => {
    const timer = setTimeout(() => {
      process.kill("SIGKILL");
      resolve();
    }, 5000);
    process.once("exit", () => {
      clearTimeout(timer);
      resolve();
    });
    if (!process.kill("SIGTERM")) {
      clearTimeout(timer);
      resolve();
    }
  });
}
async function setCode(page: Page, source: string) {
  const input = page.locator(".monaco-editor textarea.inputarea");
  await input.focus();
  await page.keyboard.press("ControlOrMeta+A");
  await page.evaluate((text) => navigator.clipboard.writeText(text), source);
  await page.keyboard.press("ControlOrMeta+V");
}
const solution =
  "def two_sum(nums: list[int], target: int) -> list[int]:\n    seen = {}\n    for i, n in enumerate(nums):\n        if target - n in seen:\n            return [seen[target - n], i]\n        seen[n] = i\n";
test.beforeAll(async () => {
  workspace = await mkdtemp(path.join(tmpdir(), "lexor-e2e-"));
  await start();
});
test.afterAll(async () => {
  await stop();
  await rm(workspace, { recursive: true, force: true });
});
test("complete practice flow, Pyright, invalid solution, timeout, and restart persistence", async ({
  page,
}) => {
  const errors: string[] = [];
  page.on("pageerror", (e) => errors.push(e.message));
  await page.goto("/");
  await expect(
    page.getByRole("heading", { name: "A little better, every day." }),
  ).toBeVisible();
  await page
    .getByRole("link", { name: "Problem library", exact: false })
    .first()
    .click();
  await expect(
    page.getByRole("link", { name: "Two Sum", exact: true }),
  ).toBeVisible();
  await page.screenshot({
    path: path.join(root, "docs/screenshots/library.png"),
    fullPage: true,
  });
  await page.getByRole("link", { name: "Two Sum", exact: true }).click();
  await expect(page.locator(".monaco-editor")).toBeVisible();
  await expect(page.locator(".monaco-editor")).toContainText("def two_sum");
  await expect(page.getByText("Pyright connected")).toBeVisible();
  await setCode(page, "import math\nmath.sq");
  await page.keyboard.press("Control+Space");
  await expect(page.locator(".suggest-widget.visible")).toBeVisible();
  await expect(page.locator(".suggest-widget.visible")).toContainText("sqrt");
  await page.keyboard.press("Escape");
  await setCode(page, solution);
  await expect(page.getByText("All changes saved")).toBeVisible();
  await page.getByRole("button", { name: "Run tests", exact: true }).click();
  await expect(page.getByTestId("execution-status")).toHaveText("Accepted");
  await expect(page.getByText("2 of 2 tests passed")).toBeVisible();
  await page.getByRole("button", { name: "Submit solution" }).click();
  await expect(page.getByText("4 of 4 tests passed")).toBeVisible();
  await page.screenshot({
    path: path.join(root, "docs/screenshots/practice.png"),
    fullPage: true,
  });
  await expect(page.getByTestId("execution-status")).toHaveText("Accepted");
  await page.goto("/");
  await expect(
    page.locator(".metric").first().locator(".metric-value"),
  ).toContainText("1");
  await page.screenshot({
    path: path.join(root, "docs/screenshots/dashboard.png"),
    fullPage: true,
  });
  await page.goto("/submissions");
  await expect(page.getByText("Accepted", { exact: true })).toBeVisible();
  await stop();
  await start();
  await page.goto("/practice/arrays.two-sum");
  await expect(page.locator(".monaco-editor")).toContainText(
    "target - n in seen",
  );
  await setCode(page, "def two_sum(nums, target):\n    return [0, 0]\n");
  await page.getByRole("button", { name: "Submit solution" }).click();
  await expect(page.getByTestId("execution-status")).toHaveText("Wrong Answer");
  await setCode(
    page,
    "def two_sum(nums, target):\n    while True:\n        pass\n",
  );
  await page.getByRole("button", { name: "Run tests", exact: true }).click();
  await expect(page.getByTestId("execution-status")).toHaveText(
    "Time Limit Exceeded",
    { timeout: 25_000 },
  );
  await setCode(page, solution);
  await expect(page.getByText("All changes saved")).toBeVisible();
  await page.goto("/submissions");
  await expect(page.getByText("Wrong Answer", { exact: true })).toBeVisible();
  expect(errors).toEqual([]);
});
test("problem form create, filter, edit raw JSON, and delete", async ({
  page,
}) => {
  await page.goto("/problems");
  await page.getByRole("link", { name: "New problem" }).click();
  await page
    .getByLabel("Title", { exact: true })
    .fill("Original API Challenge");
  await page
    .getByLabel("Description", { exact: true })
    .fill("Return the sum of the supplied numbers.");
  await page.getByRole("button", { name: "Save problem" }).first().click();
  await expect(
    page.getByRole("link", { name: "Original API Challenge", exact: true }),
  ).toBeVisible();
  await page.getByLabel("Search problems").fill("Original API");
  await expect(page.locator("tbody tr")).toHaveCount(1);
  await page.getByRole("link", { name: "Edit Original API Challenge" }).click();
  await page.getByRole("button", { name: "Raw JSON", exact: true }).click();
  const raw = page.getByLabel("Raw problem JSON");
  const data = JSON.parse(await raw.inputValue());
  data.title = "Updated API Challenge";
  await raw.fill(JSON.stringify(data, null, 2));
  await page.getByRole("button", { name: "Save problem" }).first().click();
  await expect(
    page.getByRole("link", { name: "Updated API Challenge", exact: true }),
  ).toBeVisible();
  page.once("dialog", (dialog) => dialog.accept());
  await page
    .getByRole("button", { name: "Delete Updated API Challenge" })
    .click();
  await expect(
    page.getByRole("link", { name: "Updated API Challenge", exact: true }),
  ).toHaveCount(0);
});

test("settings persist, imports preview and install, exports stay allowlisted, and assets remain local", async ({
  page,
}) => {
  const external: string[] = [];
  page.on("request", (req) => {
    if (
      !req.url().startsWith("http://127.0.0.1:8079/") &&
      !req.url().startsWith("data:")
    )
      external.push(req.url());
  });
  const source = JSON.parse(
    await readFile(path.join(root, "problems/arrays/two-sum.json"), "utf8"),
  );
  source.id = "imported.two-sum";
  source.title = "Imported Two Sum";
  const file = path.join(workspace, "external-problem.json");
  await writeFile(file, JSON.stringify(source));
  await page.goto("/settings");
  await page.getByLabel("Import source").fill(file);
  await page.getByRole("button", { name: "Preview import" }).click();
  await expect(page.getByText("1 validated problems")).toBeVisible();
  await page.getByRole("button", { name: "Confirm & import" }).click();
  await expect(page.getByText("Imported 1 problems.")).toBeVisible();
  await page.getByRole("button", { name: /Export [0-9]+ files/ }).click();
  await expect(page.getByText(/Exported [0-9]+ files to/)).toBeVisible();
  const manifest = JSON.parse(
    await readFile(path.join(workspace, ".lexor/sync/manifest.json"), "utf8"),
  );
  expect(manifest.format).toBe("lexor-progress");
  const changed = page.waitForResponse(
    (r) =>
      r.url().endsWith("/api/v1/settings") &&
      r.request().method() === "PUT" &&
      r.status() === 200,
  );
  await page.getByLabel("Workbench theme").selectOption("light");
  await changed;
  await page.reload();
  await expect(page.getByLabel("Workbench theme")).toHaveValue("light");
  await page.goto("/problems");
  await expect(
    page.getByRole("link", { name: "Imported Two Sum", exact: true }),
  ).toBeVisible();
  expect(external).toEqual([]);
});

test("light theme stays synchronized across settings, header, editor, and reloads", async ({
  page,
}) => {
  await page.goto("/settings");
  const selector = page.getByLabel("Workbench theme");
  await expect(selector).toBeEnabled();
  await selector.selectOption("light");
  await expect(page.locator("html")).toHaveAttribute("data-theme", "light");
  await expect(selector).toHaveValue("light");
  await expect(selector).toBeEnabled();
  const before = await page.evaluate(async () =>
    (await fetch("/api/v1/settings")).json(),
  );

  await page.getByRole("button", { name: "Switch to dark theme" }).click();
  await expect(selector).toHaveValue("dark");
  await expect(page.locator("html")).toHaveAttribute("data-theme", "dark");
  await page.getByRole("button", { name: "Switch to light theme" }).click();
  await expect(selector).toHaveValue("light");
  await page.reload();
  await expect(selector).toHaveValue("light");
  await expect(page.locator("html")).toHaveAttribute("data-theme", "light");
  const after = await page.evaluate(async () =>
    (await fetch("/api/v1/settings")).json(),
  );
  expect(after.editorFontSize).toBe(before.editorFontSize);
  expect(after.editorTheme).toBe("light");
  await expect(page.locator(".diagnostic").first()).toBeVisible();
  await page.screenshot({
    path: path.join(root, "docs/screenshots/settings-light.png"),
    fullPage: true,
  });

  await page.goto("/practice/arrays.two-sum");
  await expect(page.locator(".monaco-editor.vs")).toBeVisible();
  await expect(page.locator(".monaco-editor")).toContainText("def two_sum");
  await expect(page.getByText("Pyright connected")).toBeVisible();
  await page.screenshot({
    path: path.join(root, "docs/screenshots/practice-light.png"),
    fullPage: true,
  });
  for (const [route, name, ready] of [
    ["/", "dashboard-light", ".dashboard-hero"],
    ["/problems", "library-light", ".problem-table"],
  ]) {
    await page.goto(route);
    await expect(page.locator(ready)).toBeVisible();
    await page.screenshot({
      path: path.join(root, `docs/screenshots/${name}.png`),
      fullPage: true,
    });
  }
  await page.setViewportSize({ width: 390, height: 844 });
  await expect(
    page.getByRole("button", { name: "Switch to dark theme" }),
  ).toBeVisible();
  expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBe(
    390,
  );

  await page.route("**/api/v1/settings", (route) =>
    route.request().method() === "PUT"
      ? route.fulfill({
          status: 500,
          contentType: "application/json",
          body: JSON.stringify({
            error: { message: "Cannot save appearance" },
          }),
        })
      : route.continue(),
  );
  await page.getByRole("button", { name: "Switch to dark theme" }).click();
  await expect(page.getByRole("alert")).toContainText("Cannot save appearance");
  await expect(page.locator("html")).toHaveAttribute("data-theme", "light");
});
