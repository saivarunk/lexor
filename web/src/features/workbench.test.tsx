import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import seed from "../../../problems/arrays/two-sum.json";
import Problems from "./Problems";
import ProblemEditor from "./ProblemEditor";
import Practice from "./Practice";
import Submissions from "./Submissions";
import Results from "../components/Results";
import { api, post, type Execution, type ProblemView } from "../services/api";
vi.mock("../services/api", async (original) => ({
  ...(await original<typeof import("../services/api")>()),
  api: vi.fn(),
  post: vi.fn(),
}));
vi.mock("../components/CodeEditor", () => ({
  default: ({
    value,
    onChange,
  }: {
    value: string;
    onChange: (v: string) => void;
  }) => (
    <textarea
      aria-label="Python code"
      value={value}
      onChange={(e) => onChange(e.target.value)}
    />
  ),
}));
const problem = {
  ...seed,
  revision: "abc",
  file: "arrays/two-sum.json",
  status: "unsolved",
} as ProblemView;
const execution: Execution = {
  id: "submission-1",
  problemId: problem.id,
  language: "python",
  source: "def two_sum(nums,target): return [0,1]",
  mode: "submit",
  status: "Accepted",
  passed: 1,
  total: 1,
  durationMs: 12,
  memoryBytes: 0,
  stdout: "hello",
  stderr: "",
  createdAt: "2026-10-19T04:00:00Z",
  results: [
    {
      name: "Example 1",
      status: "Accepted",
      hidden: false,
      durationMs: 1,
      actual: [0, 1],
      expected: [0, 1],
    },
  ],
};
function mount(ui: React.ReactNode, path = "/") {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  return render(
    <QueryClientProvider client={client}>
      <MemoryRouter initialEntries={[path]}>{ui}</MemoryRouter>
    </QueryClientProvider>,
  );
}
beforeEach(() => {
  localStorage.clear();
  vi.mocked(api).mockReset();
  vi.mocked(post).mockReset();
});
afterEach(cleanup);
it("filters library using search and difficulty", async () => {
  vi.mocked(api).mockResolvedValue([
    problem,
    {
      ...problem,
      id: "arrays.maximum-subarray",
      title: "Maximum Subarray",
      difficulty: "medium",
    },
  ]);
  mount(<Problems />);
  await screen.findByText("Two Sum");
  await userEvent.type(screen.getByLabelText("Search problems"), "maximum");
  expect(screen.queryByText("Two Sum")).not.toBeInTheDocument();
  expect(screen.getByText("Maximum Subarray")).toBeInTheDocument();
  await userEvent.clear(screen.getByLabelText("Search problems"));
  await userEvent.selectOptions(
    screen.getByLabelText("Filter difficulty"),
    "easy",
  );
  expect(screen.getByText("Two Sum")).toBeInTheDocument();
  expect(screen.queryByText("Maximum Subarray")).not.toBeInTheDocument();
});
it("saves structured problem and displays schema feedback", async () => {
  vi.mocked(api).mockRejectedValue(
    new Error("schema validation: title is required"),
  );
  mount(<ProblemEditor />);
  await userEvent.type(
    screen.getByLabelText("Title", { exact: true }),
    "An original challenge",
  );
  await userEvent.click(
    screen.getAllByRole("button", { name: "Save problem" })[0],
  );
  expect(await screen.findByRole("alert")).toHaveTextContent(
    "schema validation",
  );
  expect(api).toHaveBeenCalledWith(
    "/problems",
    expect.objectContaining({
      method: "POST",
      body: expect.stringContaining("An original challenge"),
    }),
  );
});
it("keeps invalid raw JSON for correction", async () => {
  mount(<ProblemEditor />);
  await userEvent.click(screen.getByRole("button", { name: "Raw JSON" }));
  await userEvent.clear(screen.getByLabelText("Raw problem JSON"));
  await userEvent.type(screen.getByLabelText("Raw problem JSON"), "invalid");
  await userEvent.click(
    screen.getAllByRole("button", { name: "Save problem" })[0],
  );
  expect(await screen.findByRole("alert")).toBeInTheDocument();
  expect(api).not.toHaveBeenCalled();
});
it("restores and saves drafts and renders execution results", async () => {
  vi.mocked(api).mockImplementation(async (path) => {
    if (path === "/settings")
      return { theme: "dark", editorTheme: "dark", editorFontSize: 14 };
    if (path.startsWith("/problems/")) return problem;
    if (path.startsWith("/drafts/"))
      return {
        problemId: problem.id,
        language: "python",
        source: "saved draft",
        updatedAt: "2026-10-19T00:00:00Z",
      };
    return undefined;
  });
  vi.mocked(post).mockResolvedValue(execution);
  mount(
    <Routes>
      <Route path="/practice/:id" element={<Practice />} />
    </Routes>,
    `/practice/${problem.id}`,
  );
  const editor = await screen.findByLabelText("Python code");
  expect(editor).toHaveValue("saved draft");
  await userEvent.clear(editor);
  await userEvent.type(editor, "new draft");
  await waitFor(() =>
    expect(api).toHaveBeenCalledWith(
      `/drafts/${problem.id}?language=python`,
      expect.objectContaining({
        method: "PUT",
        body: expect.stringContaining("new draft"),
      }),
    ),
  );
  expect(localStorage.getItem(`lexor:draft:v1:${problem.id}:python`)).toContain(
    "new draft",
  );
  await userEvent.click(screen.getByRole("button", { name: "Run tests" }));
  expect(await screen.findByTestId("execution-status")).toHaveTextContent(
    "Accepted",
  );
  expect(post).toHaveBeenCalledWith(
    "/executions/run",
    expect.objectContaining({ source: "new draft" }),
  );
});
it("redacts hidden test output in result views", async () => {
  mount(
    <Results
      result={{
        ...execution,
        results: [
          {
            name: "Hidden test 1",
            hidden: true,
            status: "Wrong Answer",
            durationMs: 2,
          },
        ],
      }}
    />,
  );
  expect(
    screen.getByText("Hidden-test values and console output are redacted."),
  ).toBeInTheDocument();
  expect(screen.queryByText("EXPECTED")).not.toBeInTheDocument();
  await userEvent.click(screen.getByRole("button", { name: "Console" }));
  expect(screen.getByText("hello")).toBeInTheDocument();
});
it("opens submission history with source and results", async () => {
  vi.mocked(api).mockImplementation(async (path) =>
    path === "/submissions/submission-1" ? execution : [execution],
  );
  mount(<Submissions />);
  await userEvent.click(
    await screen.findByRole("button", { name: problem.id }),
  );
  expect(await screen.findByText(execution.source!)).toBeInTheDocument();
  expect(screen.getByTestId("execution-status")).toHaveTextContent("Accepted");
});

it("keeps raw editing available when schema validation rejects a mode switch", async () => {
  vi.mocked(api).mockRejectedValue(
    new Error("schema validation: metadata is required"),
  );
  mount(<ProblemEditor />);
  await userEvent.click(screen.getByRole("button", { name: "Raw JSON" }));
  await userEvent.clear(screen.getByLabelText("Raw problem JSON"));
  await userEvent.type(screen.getByLabelText("Raw problem JSON"), "null");
  await userEvent.click(
    screen.getByRole("button", { name: "Structured editor" }),
  );
  expect(await screen.findByRole("alert")).toHaveTextContent(
    "metadata is required",
  );
  expect(screen.getByLabelText("Raw problem JSON")).toHaveValue("null");
});
