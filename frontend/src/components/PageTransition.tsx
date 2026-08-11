import { useEffect, useState, type ReactNode } from "react";
import { useLocation } from "react-router-dom";
import { useMotion } from "../motion/MotionContext";

/** Soft fade/slide on route change when motion is enabled. */
export function PageTransition({ children }: { children: ReactNode }) {
  const location = useLocation();
  const { motionEnabled } = useMotion();
  const [visible, setVisible] = useState(true);
  const [key, setKey] = useState(location.pathname);

  useEffect(() => {
    if (!motionEnabled) {
      setKey(location.pathname);
      setVisible(true);
      return;
    }
    setVisible(false);
    const t = window.setTimeout(() => {
      setKey(location.pathname);
      setVisible(true);
    }, 120);
    return () => window.clearTimeout(t);
  }, [location.pathname, motionEnabled]);

  if (!motionEnabled) return <>{children}</>;

  return (
    <div
      key={key}
      className={`page-transition ${visible ? "page-transition-in" : "page-transition-out"}`}
    >
      {children}
    </div>
  );
}
