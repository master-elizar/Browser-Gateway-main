import type { ButtonHTMLAttributes, InputHTMLAttributes, ReactNode, SelectHTMLAttributes, TextareaHTMLAttributes } from "react";

export function Button({
  variant = "primary",
  className = "",
  children,
  type = "button",
  ...props
}: ButtonHTMLAttributes<HTMLButtonElement> & {
  variant?: "primary" | "ghost" | "danger" | "icon";
}) {
  const base =
    variant === "primary"
      ? "btn-primary"
      : variant === "danger"
        ? "btn-danger"
        : variant === "icon"
          ? "btn-icon"
          : "btn-ghost";
  return (
    <button type={type} className={`${base} ${className}`.trim()} {...props}>
      {children}
    </button>
  );
}

export function Input(props: InputHTMLAttributes<HTMLInputElement>) {
  return <input className={`input-field ${props.className ?? ""}`.trim()} {...props} />;
}

export function Select(props: SelectHTMLAttributes<HTMLSelectElement>) {
  return <select className={`input-field ${props.className ?? ""}`.trim()} {...props} />;
}

export function Textarea(props: TextareaHTMLAttributes<HTMLTextAreaElement>) {
  return <textarea className={`input-field ${props.className ?? ""}`.trim()} {...props} />;
}

export function Field({
  label,
  hint,
  children,
}: {
  label: string;
  hint?: string;
  children: ReactNode;
}) {
  return (
    <label className="block">
      <span className="field-label">{label}</span>
      {children}
      {hint && <span className="mt-1 block text-xs text-[var(--color-muted)]">{hint}</span>}
    </label>
  );
}

export function Segmented<T extends string>({
  options,
  value,
  onChange,
  fullWidth = false,
}: {
  options: { value: T; label: string }[];
  value: T;
  onChange: (value: T) => void;
  /** Stretch to fill the container with equal-width segments (e.g. a form's own tab bar). */
  fullWidth?: boolean;
}) {
  return (
    <div
      className={`${fullWidth ? "flex" : "inline-flex"} rounded-[var(--radius-md)] border border-[var(--color-line)] bg-[var(--color-panel-2)] p-1`}
    >
      {options.map((opt) => {
        const active = opt.value === value;
        return (
          <button
            key={opt.value}
            type="button"
            onClick={() => onChange(opt.value)}
            aria-pressed={active}
            className={[
              fullWidth ? "flex-1" : "",
              "rounded-[calc(var(--radius-md)-4px)] px-4 py-1.5 text-sm font-medium transition",
              active
                ? "bg-[var(--color-panel)] text-[var(--color-snow)] shadow-[var(--shadow-sm)]"
                : "text-[var(--color-fog)] hover:text-[var(--color-snow)]",
            ].join(" ")}
          >
            {opt.label}
          </button>
        );
      })}
    </div>
  );
}

// A standalone pressed/unpressed button (e.g. a view-mode switch rendered as several
// independent buttons rather than one connected group) -- for a connected set of mutually
// exclusive options, prefer Segmented above instead.
export function ToolButton({
  children,
  active,
  disabled,
  title,
  onClick,
}: {
  children: ReactNode;
  active?: boolean;
  disabled?: boolean;
  title?: string;
  onClick?: () => void;
}) {
  return (
    <button
      type="button"
      disabled={disabled}
      title={title}
      onClick={disabled ? undefined : onClick}
      className={[
        "rounded-[var(--radius-md)] border px-3 py-1.5 text-sm transition",
        disabled
          ? "cursor-not-allowed border-[var(--color-line)] bg-[var(--color-panel)]/20 text-[var(--color-muted)] opacity-60"
          : active
            ? "border-[var(--color-signal)]/60 bg-[var(--color-signal-dim)] text-[var(--color-signal-2)] shadow-[var(--shadow-sm)]"
            : "border-[var(--color-line)] bg-[var(--color-panel)]/40 text-[var(--color-fog)] hover:border-[var(--color-line-strong)] hover:bg-[var(--color-surface-hover)] hover:text-[var(--color-snow)]",
      ].join(" ")}
    >
      {children}
    </button>
  );
}

export function Toggle({
  checked,
  onChange,
  label,
  description,
}: {
  checked: boolean;
  onChange: () => void;
  label: string;
  description?: string;
}) {
  return (
    <button
      type="button"
      role="switch"
      aria-checked={checked}
      onClick={onChange}
      className="flex w-full items-center justify-between gap-4 rounded-[var(--radius-md)] border border-transparent px-1 py-2 text-left transition hover:bg-[var(--color-surface-hover)]/60"
    >
      <span className="min-w-0">
        <span className="block text-sm text-[var(--color-snow)]">{label}</span>
        {description && (
          <span className="mt-0.5 block text-xs text-[var(--color-muted)]">{description}</span>
        )}
      </span>
      <span className="toggle" data-on={checked ? "true" : "false"}>
        <span className="toggle-knob" />
      </span>
    </button>
  );
}

export function Badge({
  children,
  tone = "neutral",
}: {
  children: ReactNode;
  tone?: "neutral" | "success" | "warn" | "danger" | "info" | "accent";
}) {
  return <span className={`badge badge-${tone}`}>{children}</span>;
}

export function Alert({
  children,
  tone = "info",
}: {
  children: ReactNode;
  tone?: "info" | "success" | "warn" | "danger";
}) {
  return <div className={`alert alert-${tone}`}>{children}</div>;
}

export function Card({
  children,
  className = "",
  solid = false,
}: {
  children: ReactNode;
  className?: string;
  solid?: boolean;
}) {
  return <section className={`${solid ? "ui-card-solid" : "ui-card"} ${className}`.trim()}>{children}</section>;
}

export function CardHeader({
  title,
  description,
  action,
}: {
  title: string;
  description?: string;
  action?: ReactNode;
}) {
  return (
    <div className="flex items-start justify-between gap-3 border-b border-[var(--color-line)] px-6 py-5">
      <div>
        <h2 className="section-title">{title}</h2>
        {description && <p className="section-desc">{description}</p>}
      </div>
      {action}
    </div>
  );
}

export function CardBody({ children, className = "" }: { children: ReactNode; className?: string }) {
  return <div className={`px-6 py-5 ${className}`.trim()}>{children}</div>;
}

export function PageHeader({
  title,
  subtitle,
  actions,
}: {
  title: string;
  subtitle?: string;
  actions?: ReactNode;
}) {
  return (
    <div className="page-header animate-fade-in">
      <div>
        <h1 className="page-title">{title}</h1>
        {subtitle && <p className="page-subtitle">{subtitle}</p>}
      </div>
      {actions && <div className="flex flex-wrap items-center gap-2">{actions}</div>}
    </div>
  );
}

export function EmptyState({
  title,
  description,
  action,
  icon,
}: {
  title: string;
  description?: string;
  action?: ReactNode;
  icon?: ReactNode;
}) {
  return (
    <div className="flex flex-col items-center justify-center px-6 py-14 text-center">
      {icon && <div className="mb-3 text-[var(--color-muted)]">{icon}</div>}
      <div className="text-sm font-medium text-[var(--color-snow)]">{title}</div>
      {description && <p className="mt-1 max-w-sm text-xs text-[var(--color-muted)]">{description}</p>}
      {action && <div className="mt-4">{action}</div>}
    </div>
  );
}

export function Skeleton({ className = "" }: { className?: string }) {
  return <div className={`skeleton ${className}`.trim()} />;
}

export function StatusDot({ tone = "neutral" }: { tone?: "success" | "warn" | "danger" | "neutral" | "accent" }) {
  const color =
    tone === "success"
      ? "bg-[var(--color-success)]"
      : tone === "warn"
        ? "bg-[var(--color-warn)]"
        : tone === "danger"
          ? "bg-[var(--color-danger)]"
          : tone === "accent"
            ? "bg-[var(--color-signal-2)]"
            : "bg-[var(--color-muted)]";
  return <span className={`inline-block size-1.5 rounded-full ${color}`} />;
}
