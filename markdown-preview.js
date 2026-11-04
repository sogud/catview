(async () => {
  const root = document.documentElement;
  const blocks = Array.from(document.querySelectorAll('pre > code.language-mermaid'));
  const diagrams = blocks.map(code => ({ pre: code.parentElement, source: code.textContent }));
  const media = matchMedia('(prefers-color-scheme: dark)');
  let rendering = false, pending = false;

  function theme() {
    const style = getComputedStyle(root);
    const color = name => style.getPropertyValue(name).trim();
    const dark = root.dataset.theme === 'dark' || (root.dataset.theme === 'auto' && media.matches);
    const paper = color('--paper'), ink = color('--ink'), panel = color('--panel');
    const line = color('--line'), accent = color('--secondary');
    return {
      startOnLoad: false, securityLevel: 'strict', suppressErrorRendering: true,
      maxTextSize: 50000, maxEdges: 500, theme: 'base', look: 'classic', htmlLabels: false,
      fontFamily: '-apple-system, BlinkMacSystemFont, "PingFang SC", sans-serif',
      secure: ['secure', 'securityLevel', 'startOnLoad', 'maxTextSize', 'maxEdges',
        'suppressErrorRendering', 'htmlLabels', 'themeCSS', 'fontFamily'],
      themeVariables: {
        darkMode: dark, background: paper, primaryColor: panel, primaryTextColor: ink,
        primaryBorderColor: accent, lineColor: accent, textColor: ink,
        secondaryColor: panel, secondaryTextColor: ink, tertiaryColor: paper,
        tertiaryTextColor: ink, nodeBorder: accent, mainBkg: panel,
        clusterBkg: paper, clusterBorder: line, edgeLabelBackground: paper,
        actorBkg: panel, actorBorder: accent, actorTextColor: ink, actorLineColor: line,
        signalColor: accent, signalTextColor: ink, labelBoxBkgColor: panel,
        labelBoxBorderColor: line, labelTextColor: ink, loopTextColor: ink,
        noteBkgColor: panel, noteBorderColor: line, noteTextColor: ink,
        activationBkgColor: panel, activationBorderColor: accent,
        fontSize: '14px'
      },
      flowchart: { htmlLabels: false, curve: 'basis', nodeSpacing: 36, rankSpacing: 48, padding: 14 },
      sequence: { useMaxWidth: true, mirrorActors: false, actorMargin: 40, messageMargin: 34 }
    };
  }

  async function render() {
    if (rendering) { pending = true; return; }
    rendering = true;
    do {
      pending = false;
      if (diagrams.length) mermaid.initialize(theme());
      for (const [index, diagram] of diagrams.entries()) {
        try {
          if (diagram.source.length > 50000) throw new Error('Diagram exceeds 50,000 characters');
          const { svg } = await mermaid.render('catview-diagram-' + index, diagram.source);
          const figure = document.createElement('figure');
          figure.className = 'cv-diagram';
          figure.setAttribute('aria-label', 'Mermaid');
          figure.innerHTML = svg;
          const graphic = figure.querySelector('svg');
          if (diagram.figure) diagram.figure.replaceWith(figure);
          else diagram.pre.replaceWith(figure);
          diagram.figure = figure;
          if (graphic) {
            const width = graphic.viewBox.baseVal.width;
            const available = figure.clientWidth - 32;
            // Fit modest charts; keep large charts scrollable at readable text size.
            if (width > 0) graphic.style.width = (width <= available / 0.75 ? Math.min(width, available) : width) + 'px';
          }
        } catch (_) {
          if (!diagram.error) {
            const error = document.createElement('p');
            error.className = 'cv-diagram-error';
            error.textContent = catviewPreview.error;
            diagram.pre.before(error);
            diagram.error = error;
          }
        }
      }
    } while (pending);
    rendering = false;
  }

  await document.fonts.ready;
  await render();
  if (catviewPreview.fragment) {
    const target = document.getElementById(catviewPreview.fragment) ||
      Array.from(document.getElementsByName(catviewPreview.fragment))[0];
    target?.scrollIntoView();
  }
  if (diagrams.length) media.addEventListener('change', () => {
    if (root.dataset.theme === 'auto') render();
  });
})();
