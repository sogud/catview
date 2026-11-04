(async () => {
  const root = document.documentElement;
  const blocks = Array.from(document.querySelectorAll('pre > code.language-mermaid'));
  const diagrams = blocks.map(code => ({ pre: code.parentElement, source: code.textContent }));
  const media = matchMedia('(prefers-color-scheme: dark)');
  let rendering = false, pending = false;
  function fitDiagram(figure) {
    const graphic = figure.querySelector('svg');
    if (!graphic) return;
    const width = graphic.viewBox.baseVal.width;
    const available = Math.max(1, figure.clientWidth - 16);
    // Fit modest charts; keep large charts scrollable at readable text size.
    if (width > 0) graphic.style.width = (width <= available / 0.75 ? Math.min(width, available) : width) + 'px';
  }
  const resize = new ResizeObserver(entries => {
    for (const entry of entries) fitDiagram(entry.target);
  });

  function theme() {
    const style = getComputedStyle(root);
    const color = name => style.getPropertyValue(name).trim();
    const dark = root.dataset.theme === 'dark' || (root.dataset.theme === 'auto' && media.matches);
    const paper = color('--paper');
    // Match the quiet blue diagrams of Codex in both appearances.
    const panel = dark ? '#062C4C' : '#E0EDF9';
    const ink = dark ? '#9CCFFF' : '#006BD6';
    const border = dark ? '#294D67' : '#D0D9E2';
    const line = dark ? '#8A929A' : '#8B9299';
    const label = dark ? '#071B2A' : paper;
    return {
      startOnLoad: false, securityLevel: 'strict', suppressErrorRendering: true,
      maxTextSize: 50000, maxEdges: 500, theme: 'base', look: 'classic', htmlLabels: false,
      fontFamily: '-apple-system, BlinkMacSystemFont, "PingFang SC", sans-serif',
      secure: ['secure', 'securityLevel', 'startOnLoad', 'maxTextSize', 'maxEdges',
        'suppressErrorRendering', 'htmlLabels', 'themeCSS', 'fontFamily'],
      themeVariables: {
        darkMode: dark, background: paper, primaryColor: panel, primaryTextColor: ink,
        primaryBorderColor: border, lineColor: line, textColor: ink,
        secondaryColor: panel, secondaryTextColor: ink, tertiaryColor: paper,
        tertiaryTextColor: ink, nodeBorder: border, mainBkg: panel,
        clusterBkg: paper, clusterBorder: border, edgeLabelBackground: label,
        actorBkg: panel, actorBorder: border, actorTextColor: ink, actorLineColor: border,
        signalColor: line, signalTextColor: ink, labelBoxBkgColor: label,
        labelBoxBorderColor: border, labelTextColor: ink, loopTextColor: ink,
        noteBkgColor: panel, noteBorderColor: border, noteTextColor: ink,
        activationBkgColor: panel, activationBorderColor: border,
        fontSize: '15px'
      },
      themeCSS: `.nodeLabel,.node .label,.cluster-label,.actor{font-weight:600}
        .flowchart-link{stroke-width:1.2px;stroke-linecap:round;stroke-linejoin:round}
        .edgeLabel{color:${ink};font-weight:500}
        .labelBkg{fill:${label};opacity:1}
        .messageLine0,.messageLine1{stroke-width:1.2px}
        .actor-line{stroke-width:1px}`,
      flowchart: { htmlLabels: false, curve: 'linear', nodeSpacing: 40, rankSpacing: 56, padding: 20 },
      sequence: { useMaxWidth: true, mirrorActors: false, actorMargin: 44, messageMargin: 36, width: 150, height: 52 }
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
          if (graphic) {
            // Round rectangular nodes, preserving circles, decisions and pills.
            for (const rect of graphic.querySelectorAll('.node rect, rect.actor')) {
              if (Number(rect.getAttribute('rx') || 0) <= 6) {
                rect.setAttribute('rx', '14');
                rect.setAttribute('ry', '14');
              }
            }
          }
          if (diagram.figure) {
            resize.unobserve(diagram.figure);
            diagram.figure.replaceWith(figure);
          }
          else diagram.pre.replaceWith(figure);
          diagram.figure = figure;
          fitDiagram(figure);
          resize.observe(figure);
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
