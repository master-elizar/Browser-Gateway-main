import { useTranslation } from "react-i18next";
import { Select } from "./ui";

export function LanguageSwitch() {
  const { i18n, t } = useTranslation();
  const lang = i18n.language.startsWith("ru") ? "ru" : "en";

  return (
    <label className="flex items-center gap-2 text-xs text-[var(--color-fog)]" title={t("common.language")}>
      <span className="hidden xl:inline">{t("common.language")}</span>
      <Select
        className="!w-auto min-w-[4.5rem] py-1.5 text-xs"
        value={lang}
        onChange={(e) => {
          const next = e.target.value;
          void i18n.changeLanguage(next);
          localStorage.setItem("bg.lang", next);
        }}
        aria-label={t("common.language")}
      >
        <option value="en">EN</option>
        <option value="ru">RU</option>
      </Select>
    </label>
  );
}
