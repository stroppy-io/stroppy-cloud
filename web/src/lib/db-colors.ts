import type { DatabaseKind } from "@/api/types";

// Corporate brand colors for each database engine.
// PostgreSQL: elephant blue — #336791
// MySQL: dolphin orange — #F29111
// Picodata: coral red — #E23956

export const DB_COLORS: Record<DatabaseKind, {
  hex: string;
  hexLight: string;
  text: string;
  accent: string;
  hexSecondary: string;
}> = {
  postgres: {
    hex: "#336791",
    hexLight: "#5B93C4",
    text: "text-[#5B93C4]",
    accent: "border-[#336791]/50 bg-[#336791]/[0.08]",
    hexSecondary: "#4A7FAF",
  },
  mysql: {
    hex: "#F29111",
    hexLight: "#F2A94B",
    text: "text-[#F2A94B]",
    accent: "border-[#F29111]/50 bg-[#F29111]/[0.08]",
    hexSecondary: "#E8A030",
  },
  picodata: {
    hex: "#E23956",
    hexLight: "#F06580",
    text: "text-[#F06580]",
    accent: "border-[#E23956]/50 bg-[#E23956]/[0.08]",
    hexSecondary: "#C7304A",
  },
};
