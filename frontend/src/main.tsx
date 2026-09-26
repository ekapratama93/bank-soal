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
import { GraduationCap, HistoryIcon, HouseIcon, LogOut, Settings } from "lucide-react";
import Home from "./pages/Home";
import Quiz from "./pages/Quiz";
import Result from "./pages/Result";
import History from "./pages/History";
import Admin from "./pages/Admin";
import { cn } from "@/lib/utils";
import { clearAdminToken, useAdminToken } from "@/lib/adminAuth";
import { Button } from "@/components/ui/button";
import "katex/dist/katex.min.css";
import "./index.css";

function Shell() {
  // Beranda tampil penuh (landing page); halaman ujian butuh kolom lebih
  // lebar untuk navigator soal; halaman lain tetap dalam kolom sempit.
  const location = useLocation();
  const isHome = location.pathname === "/";
  const isQuiz = location.pathname.startsWith("/quiz/");
  const isAdmin = location.pathname.startsWith("/admin");
  const adminToken = useAdminToken();

  return (
    <>
      {/* Tinggi header tetap h-14: elemen sticky di Quiz/Result memakai
          offset top-14 dst. yang bergantung pada tinggi ini. */}
      {/* Padding + max-w sama dengan hero di Beranda supaya tepi logo/menu
          sejajar dengan konten; dipakai sama persis di semua halaman. */}
      <header className="border-border/60 bg-background/85 supports-[backdrop-filter]:bg-background/70 sticky top-0 z-20 h-14 border-b px-4 backdrop-blur-md sm:px-10 lg:px-16">
        <nav className="mx-auto flex h-full max-w-6xl items-center justify-between gap-3">
          <Link
            to="/"
            className="group text-foreground flex shrink-0 items-center gap-2.5 text-lg font-extrabold tracking-tight whitespace-nowrap no-underline"
          >
            <span className="bg-primary text-primary-foreground flex size-8 items-center justify-center rounded-lg shadow-sm">
              <GraduationCap className="size-5 group-hover:animate-wiggle" />
            </span>
            Bank Soal
          </Link>
          <div className="flex items-center gap-2">
            <div className="border-border/60 bg-muted/70 flex items-center gap-1 rounded-full border p-1">
              {[
                { to: "/", label: "Beranda", icon: HouseIcon, end: true },
                { to: "/riwayat", label: "Riwayat", icon: HistoryIcon, end: false },
                { to: "/admin", label: "Admin", icon: Settings, end: false },
              ].map((item) => (
                <NavLink
                  key={item.to}
                  to={item.to}
                  end={item.end}
                  aria-label={item.label}
                  className={({ isActive }) =>
                    cn(
                      "text-muted-foreground hover:text-foreground flex items-center gap-1.5 rounded-full px-2.5 py-1.5 text-sm font-semibold whitespace-nowrap no-underline transition-colors duration-200 sm:px-3",
                      isActive && "bg-card text-foreground shadow-sm"
                    )
                  }
                >
                  <item.icon className="size-4 shrink-0" />
                  <span className="hidden sm:inline">{item.label}</span>
                </NavLink>
              ))}
            </div>
            {isAdmin && adminToken && (
              <Button
                variant="destructive"
                size="sm"
                className="shrink-0 px-2.5 sm:px-4"
                aria-label="Keluar"
                title="Keluar"
                onClick={clearAdminToken}
              >
                <LogOut />
                <span className="hidden sm:inline">Keluar</span>
              </Button>
            )}
          </div>
        </nav>
      </header>
      <main
        className={cn(
          "w-full",
          !isHome && !isQuiz && !isAdmin && "mx-auto max-w-3xl px-4 py-6",
          isQuiz && "mx-auto max-w-5xl px-4 py-6",
          // Padding sama dengan header; max-w-6xl ada di dalam Admin supaya
          // tepi konten sejajar dengan logo/menu.
          isAdmin && "px-4 py-6 sm:px-10 lg:px-16"
        )}
      >
        {/* key per path: setiap pindah halaman konten masuk dengan fade-up */}
        <div key={location.pathname} className="animate-fade-up">
          <Outlet />
        </div>
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