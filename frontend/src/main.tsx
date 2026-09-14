import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import {
  Link,
  NavLink,
  Outlet,
  RouterProvider,
  createBrowserRouter,
  useLocation,
} from "react-router-dom";
import { GraduationCap } from "lucide-react";
import Home from "./pages/Home";
import Quiz from "./pages/Quiz";
import Result from "./pages/Result";
import History from "./pages/History";
import Admin from "./pages/Admin";
import { cn } from "@/lib/utils";
import "katex/dist/katex.min.css";
import "./index.css";

function Shell() {
  // Beranda tampil penuh (landing page); halaman ujian butuh kolom lebih
  // lebar untuk navigator soal; halaman lain tetap dalam kolom sempit.
  const location = useLocation();
  const isHome = location.pathname === "/";
  const isQuiz = location.pathname.startsWith("/quiz/");
  const isAdmin = location.pathname.startsWith("/admin");

  return (
    <>
      <nav className="from-primary to-primary/80 sticky top-0 z-20 flex items-center justify-between gap-2 bg-gradient-to-r px-3 py-3 shadow-sm sm:px-6">
        <Link
          to="/"
          className="flex shrink-0 items-center gap-2 text-base font-extrabold whitespace-nowrap text-primary-foreground no-underline sm:text-lg"
        >
          <GraduationCap className="size-6 shrink-0" />
          Bank Soal
        </Link>
        <div className="flex gap-0.5 sm:gap-1">
          {[
            { to: "/", label: "Beranda", end: true },
            { to: "/riwayat", label: "Riwayat", end: false },
            { to: "/admin", label: "Admin", end: false },
          ].map((item) => (
            <NavLink
              key={item.to}
              to={item.to}
              end={item.end}
              className={({ isActive }) =>
                cn(
                  "rounded-full px-2 py-1.5 text-xs font-semibold whitespace-nowrap text-primary-foreground/80 no-underline transition-colors hover:text-primary-foreground sm:px-3 sm:text-sm",
                  isActive && "bg-primary-foreground/20 text-primary-foreground"
                )
              }
            >
              {item.label}
            </NavLink>
          ))}
        </div>
      </nav>
      <main
        className={cn(
          "w-full",
          !isHome && !isQuiz && !isAdmin && "mx-auto max-w-3xl px-4 py-6",
          isQuiz && "mx-auto max-w-5xl px-4 py-6",
          isAdmin && "mx-auto max-w-4xl px-4 py-6"
        )}
      >
        <Outlet />
      </main>
    </>
  );
}

// Data router (createBrowserRouter) dipakai supaya halaman ujian bisa menahan
// navigasi keluar lewat useBlocker — itu tidak tersedia di <BrowserRouter>.
const router = createBrowserRouter([
  {
    element: <Shell />,
    children: [
      { path: "/", element: <Home /> },
      { path: "/quiz/:quizId", element: <Quiz /> },
      { path: "/result/:quizId", element: <Result /> },
      { path: "/riwayat", element: <History /> },
      { path: "/admin", element: <Admin /> },
    ],
  },
]);

function App() {
  return <RouterProvider router={router} />;
}

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <App />
  </StrictMode>
);