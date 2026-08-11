import type { ReactNode, TableHTMLAttributes } from "react";

export function DataTable({
  children,
  className = "",
  ...props
}: TableHTMLAttributes<HTMLTableElement> & { children: ReactNode }) {
  return (
    <div className="ui-table-wrap">
      <table className={`ui-table ${className}`.trim()} {...props}>
        {children}
      </table>
    </div>
  );
}
