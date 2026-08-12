import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { BrowserRouter } from "react-router-dom";
import App from "./App";
import { AuthProvider } from "./auth/AuthContext";
import { ToastProvider } from "./components/ui/Toast";
import { HackerAmbience } from "./components/HackerAmbience";
import { ThemeProvider } from "./theme/ThemeContext";
import { MotionProvider } from "./motion/MotionContext";
import "./i18n";
import "./fonts.css";
import "./index.css";

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <BrowserRouter>
      <ThemeProvider>
        <MotionProvider>
          <AuthProvider>
            <ToastProvider>
              <HackerAmbience />
              <div className="relative z-10 min-h-full">
                <App />
              </div>
            </ToastProvider>
          </AuthProvider>
        </MotionProvider>
      </ThemeProvider>
    </BrowserRouter>
  </StrictMode>,
);
