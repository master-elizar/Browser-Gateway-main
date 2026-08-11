import i18n from "i18next";
import { initReactI18next } from "react-i18next";
import en from "./locales/en.json";
import ru from "./locales/ru.json";

const saved = localStorage.getItem("bg.lang");
const initial = saved === "ru" || saved === "en" ? saved : "en";

void i18n.use(initReactI18next).init({
  resources: {
    en: { translation: en },
    ru: { translation: ru },
  },
  lng: initial,
  fallbackLng: "en",
  interpolation: { escapeValue: false },
});

export default i18n;
