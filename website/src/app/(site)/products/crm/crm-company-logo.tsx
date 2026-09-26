import { COMPANIES } from "./crm-demo-data";

// Original demo-company marks, kept consistent across rows and linked records.
export function CRMCompanyLogo({
  companyIndex,
  size = 20,
}: {
  companyIndex: number;
  size?: number;
}) {
  const company = COMPANIES[companyIndex];
  return (
    <img
      className="cw-company-logo"
      src={`/new/crm/logos/${company.logo}.svg`}
      width={size}
      height={size}
      alt=""
    />
  );
}
