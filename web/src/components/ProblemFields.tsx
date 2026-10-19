import type { Dispatch, SetStateAction } from "react";
import type { Problem } from "../services/api";
export type FormParts = {
  examples: string;
  public: string;
  hidden: string;
  entrypoint: string;
  comparison: string;
};
export default function ProblemFields({
  problem,
  change,
  parts,
  setParts,
  setDirty,
  existing,
}: {
  problem: Problem;
  change: (patch: Partial<Problem>) => void;
  parts: FormParts;
  setParts: Dispatch<SetStateAction<FormParts>>;
  setDirty: Dispatch<SetStateAction<boolean>>;
  existing: boolean;
}) {
  return (
    <div className="problem-form">
      <FormSection
        number="01"
        title="The essentials"
        description="Give your challenge a clear identity."
      >
        <div className="form-grid">
          <Field label="Title">
            <input
              aria-label="Title"
              value={problem.title}
              onChange={(e) => change({ title: e.target.value })}
              placeholder="e.g. Two Sum"
            />
          </Field>
          <Field label="Difficulty">
            <select
              aria-label="Difficulty"
              value={problem.difficulty}
              onChange={(e) => change({ difficulty: e.target.value })}
            >
              {["easy", "medium", "hard"].map((v) => (
                <option key={v}>{v}</option>
              ))}
            </select>
          </Field>
          <Field
            label="Stable ID"
            hint="Lowercase letters, numbers, dots and hyphens. Immutable after creation."
          >
            <input
              aria-label="Stable ID"
              disabled={!!existing}
              value={problem.id}
              onChange={(e) => change({ id: e.target.value })}
            />
          </Field>
          <Field label="Slug">
            <input
              aria-label="Slug"
              value={problem.slug}
              onChange={(e) => change({ slug: e.target.value })}
            />
          </Field>
        </div>
        <Field label="Description">
          <textarea
            aria-label="Description"
            rows={5}
            value={problem.description}
            onChange={(e) => change({ description: e.target.value })}
            placeholder="Describe the task in your own words…"
          />
        </Field>
      </FormSection>
      <FormSection
        number="02"
        title="Organization"
        description="Filesystem folders derive from the ID; topics are your logical hierarchy."
      >
        <div className="form-grid">
          <Field label="Topic">
            <input
              aria-label="Topic"
              value={problem.organization.topic}
              onChange={(e) =>
                change({
                  organization: {
                    ...problem.organization,
                    topic: e.target.value,
                  },
                })
              }
            />
          </Field>
          {(["tags", "companies", "path"] as const).map((key) => (
            <Field
              key={key}
              label={`${key[0].toUpperCase() + key.slice(1)} (comma-separated)`}
            >
              <input
                aria-label={key}
                value={problem.organization[key].join(", ")}
                onChange={(e) =>
                  change({
                    organization: {
                      ...problem.organization,
                      [key]: e.target.value.split(",").map((t) => t.trim()),
                    },
                  })
                }
              />
            </Field>
          ))}
        </div>
        <Field label="Constraints (one per line)">
          <textarea
            aria-label="Constraints"
            rows={3}
            value={problem.constraints.join("\n")}
            onChange={(e) =>
              change({ constraints: e.target.value.split("\n") })
            }
          />
        </Field>
      </FormSection>
      <FormSection
        number="03"
        title="Examples & tests"
        description="Inputs use named parameters. Expected values may be any JSON value, including null."
      >
        {(["examples", "public", "hidden"] as const).map((key) => (
          <Field
            key={key}
            label={
              key === "examples"
                ? "Examples (JSON)"
                : key === "public"
                  ? "Sample test cases (JSON)"
                  : "Hidden test cases (JSON)"
            }
            hint={
              key === "hidden"
                ? "Locally owned hidden tests are not secret; their outputs are redacted from execution responses."
                : undefined
            }
          >
            <textarea
              className="code-input"
              aria-label={
                key === "public"
                  ? "Sample test cases"
                  : key === "hidden"
                    ? "Hidden test cases"
                    : "Examples"
              }
              rows={8}
              spellCheck={false}
              value={parts[key]}
              onChange={(e) => {
                setParts((p) => ({ ...p, [key]: e.target.value }));
                setDirty(true);
              }}
            />
          </Field>
        ))}
      </FormSection>
      <FormSection
        number="04"
        title="Language & evaluation"
        description="Function signatures describe types; test values always travel as JSON."
      >
        <Field label="Python entrypoint (JSON)">
          <textarea
            className="code-input"
            aria-label="Entrypoint"
            rows={8}
            value={parts.entrypoint}
            onChange={(e) => {
              setParts((p) => ({ ...p, entrypoint: e.target.value }));
              setDirty(true);
            }}
          />
        </Field>
        <Field label="Python starter code">
          <textarea
            className="code-input"
            aria-label="Starter code"
            rows={7}
            spellCheck={false}
            value={problem.languages.python?.starterCode ?? ""}
            onChange={(e) =>
              change({
                languages: {
                  ...problem.languages,
                  python: {
                    ...problem.languages.python,
                    starterCode: e.target.value,
                  },
                },
              })
            }
          />
        </Field>
        <Field
          label="Comparison options (JSON)"
          hint="Modes: exact, float, unordered. Float supports absoluteTolerance and relativeTolerance."
        >
          <textarea
            className="code-input"
            rows={3}
            aria-label="Comparison"
            value={parts.comparison}
            onChange={(e) => {
              setParts((p) => ({ ...p, comparison: e.target.value }));
              setDirty(true);
            }}
          />
        </Field>
      </FormSection>
      <FormSection
        number="05"
        title="Provenance"
        description="Keep your library portable and its origins clear."
      >
        <div className="form-grid">
          {(["author", "source", "version"] as const).map((key) => (
            <Field label={key} key={key}>
              <input
                aria-label={`Metadata ${key}`}
                value={problem.metadata[key]}
                onChange={(e) =>
                  change({
                    metadata: {
                      ...problem.metadata,
                      [key]: e.target.value,
                    },
                  })
                }
              />
            </Field>
          ))}
        </div>
      </FormSection>
    </div>
  );
}

function Field({
  label,
  hint,
  children,
}: {
  label: string;
  hint?: string;
  children: React.ReactNode;
}) {
  return (
    <label className="field">
      <span>{label}</span>
      {children}
      {hint ? <small>{hint}</small> : null}
    </label>
  );
}
function FormSection({
  number,
  title,
  description,
  children,
}: {
  number: string;
  title: string;
  description: string;
  children: React.ReactNode;
}) {
  return (
    <section className="card form-section">
      <div className="form-section-heading">
        <span>{number}</span>
        <div>
          <h3>{title}</h3>
          <p>{description}</p>
        </div>
      </div>
      <div className="form-section-fields">{children}</div>
    </section>
  );
}
