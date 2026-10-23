# Архив лендинга консоли

Лендинг консоли (`/land`) и страница «Как устроено» (`/how`) убраны: у
консоли нет своей главной, публичное лицо продукта — stroppy.io (решение в
`docs/plans/site-console.md`). Код лежит здесь как источник идей для сайта и
не собирается: пути импортов (`@components/...`) указывают на то, где файлы
жили в `web/src`.

| Файл | Что внутри |
|---|---|
| `components/landing/LandingPage.tsx` | лендинг как дека шагов с прокруткой: hero, история прогона, рейтинг, финал |
| `components/landing/RunStory.tsx` | анимированная история одного прогона: стенд, база, нагрузка, отчёт; заголовки шагов, лог, события |
| `components/landing/RunCanvas.tsx` | рамка «окна» прогона и полоса отчёта для истории |
| `components/landing/RatingScreen.tsx` | экран рейтинга на лендинге (топ из `/api/v1/public/rating`) |
| `components/landing/EndPage.tsx`, `PublicFooter.tsx` | финальный экран с призывом и футер |
| `components/landing/DeckNav.tsx` | навигация по шагам деки, подсказка прокрутки |
| `components/how/HowPage.tsx`, `RunLifecycle.tsx` | «Как устроено»: продукт, архитектура, жизненный цикл прогона |
| `components/public/ArchitectureDiagram.tsx` | схема архитектуры (сервер, Graphene, облако, агенты) |
| `components/public/PageHero.tsx` | шапка страницы с логотипом-знаком |
| `routes/land.tsx`, `routes/how.tsx` | маршруты, как они были |
| `locales/{en,ru}/how.json` | тексты «Как устроено» |
| `locales/{en,ru}/landing.json` | тексты лендинга на момент удаления |

`HeroMark` (знак с лучами) и шапка публичных экранов остались в консоли:
`web/src/components/public/HeroMark.tsx`, `SiteBar.tsx`. Тексты
`landing.*` в `web/src/locales` остались, их читают рейтинг и шапка.
