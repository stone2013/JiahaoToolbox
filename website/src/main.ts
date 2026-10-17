import './style.css';

const menuButton = document.querySelector<HTMLButtonElement>('.menu-toggle');
const nav = document.querySelector<HTMLElement>('#site-nav');
function closeMenu(restoreFocus = false): void {
  menuButton?.setAttribute('aria-expanded', 'false');
  menuButton?.setAttribute('aria-label', '打开导航菜单');
  nav?.classList.remove('is-open');
  if (restoreFocus) menuButton?.focus();
}
menuButton?.addEventListener('click', () => {
  const open = menuButton.getAttribute('aria-expanded') !== 'true';
  menuButton.setAttribute('aria-expanded', String(open));
  menuButton.setAttribute('aria-label', open ? '关闭导航菜单' : '打开导航菜单');
  nav?.classList.toggle('is-open', open);
});
nav?.querySelectorAll('a').forEach(link => link.addEventListener('click', () => closeMenu()));
document.addEventListener('click', event => {
  if (event.target instanceof Node && !document.querySelector('#header')?.contains(event.target)) closeMenu();
});
document.addEventListener('keydown', event => {
  if (event.key === 'Escape' && menuButton?.getAttribute('aria-expanded') === 'true') closeMenu(true);
});
window.matchMedia('(min-width: 801px)').addEventListener('change', () => closeMenu());

const header = document.querySelector('#header');
let headerPending = false;
function updateHeader(): void {
  header?.classList.toggle('scrolled', window.scrollY > 16);
  headerPending = false;
}
updateHeader();
window.addEventListener('scroll', () => {
  if (!headerPending) { headerPending = true; requestAnimationFrame(updateHeader); }
}, { passive: true });

const dialog = document.querySelector<HTMLDialogElement>('#screenshot-dialog');
let lastScreenshot: HTMLElement | null = null;
document.querySelectorAll<HTMLButtonElement>('[data-lightbox]').forEach(button => {
  button.addEventListener('click', () => {
    if (!dialog) return;
    const image = dialog.querySelector<HTMLImageElement>('.lightbox-image');
    const caption = dialog.querySelector('#lightbox-caption');
    if (!image || !caption || !button.dataset.lightbox) return;
    lastScreenshot = button;
    image.src = `${import.meta.env.BASE_URL}${button.dataset.lightbox}`;
    image.alt = button.dataset.caption ?? '嘉豪工具箱真实软件截图';
    caption.textContent = image.alt;
    dialog.showModal();
    document.body.style.overflow = 'hidden';
  });
});
dialog?.querySelector('.close-lightbox')?.addEventListener('click', () => dialog.close());
dialog?.addEventListener('click', event => {
  if (event.target === dialog) {
    const bounds = dialog.getBoundingClientRect();
    if (event.clientX < bounds.left || event.clientX > bounds.right || event.clientY < bounds.top || event.clientY > bounds.bottom) dialog.close();
  }
});
dialog?.addEventListener('close', () => {
  document.body.style.overflow = '';
  lastScreenshot?.focus({ preventScroll: true });
});

const reducedMotion = window.matchMedia('(prefers-reduced-motion: reduce)');
if ('IntersectionObserver' in window) {
  const reveal = new IntersectionObserver(entries => {
    entries.forEach(entry => {
      if (entry.isIntersecting) { entry.target.classList.add('visible'); reveal.unobserve(entry.target); }
    });
  }, { threshold: 0.08 });
  if (!reducedMotion.matches) {
    document.documentElement.classList.add('is-enhanced');
    document.querySelectorAll('.reveal').forEach(element => reveal.observe(element));
  }
  reducedMotion.addEventListener('change', event => {
    if (event.matches) { reveal.disconnect(); document.documentElement.classList.remove('is-enhanced'); }
  });
  const sections = new IntersectionObserver(entries => {
    for (const entry of entries) {
      if (!entry.isIntersecting) continue;
      document.querySelectorAll('#site-nav a[href^="#"]').forEach(link => {
        if (link.getAttribute('href') === `#${entry.target.id}`) link.setAttribute('aria-current', 'page');
        else link.removeAttribute('aria-current');
      });
    }
  }, { rootMargin: '-15% 0px -65% 0px', threshold: 0 });
  document.querySelectorAll('section[id]').forEach(section => sections.observe(section));
}

const video = document.querySelector<HTMLVideoElement>('#promo-video');
document.addEventListener('visibilitychange', () => { if (document.hidden) video?.pause(); });

const playButton = document.querySelector<HTMLButtonElement>('#play-promo');
const videoError = document.querySelector<HTMLElement>('#video-error');
if (video && playButton) {
  playButton.hidden = false;
  const showVideoError = (): void => { if(videoError) videoError.hidden = false; playButton.hidden = false; };
  playButton.addEventListener('click', () => {
    if (video.error) video.load();
    video.play().catch(showVideoError);
  });
  video.addEventListener('play', () => { playButton.hidden = true; if(videoError) videoError.hidden = true; });
  video.addEventListener('error', showVideoError);
  // A failed <source> does not bubble its error to the parent <video>.
  video.querySelectorAll('source').forEach(source => source.addEventListener('error', showVideoError));
}
