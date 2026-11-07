// Ambient Video Glow - Directional Glass Diffusion Controller
// High-performance, lightweight color sampling strictly restricted to frosted/liquid glass controls.
// Automatically deactivates on pause and on non-playing pages with zero residual light.

(function () {
  'use strict';

  class AmbientGlowController {
    constructor() {
      this.video = null;
      this.sampleCanvas = document.createElement('canvas');
      this.sampleCanvas.width = 16;
      this.sampleCanvas.height = 16;
      this.sampleCtx = this.sampleCanvas.getContext('2d', { willReadFrequently: true });

      // Smoothed RGB vectors for left and top edges
      this.currentLeft = [20, 22, 32];
      this.currentTop = [20, 22, 32];
      this.targetLeft = [20, 22, 32];
      this.targetTop = [20, 22, 32];

      this.isRunning = false;
      this.rafId = null;
      this.lastSampleTime = 0;
      this.sampleIntervalMs = 60; // ~16fps: optimum smoothness with zero main-thread burden

      this.initVisibilityListener();
    }

    initVisibilityListener() {
      document.addEventListener('visibilitychange', () => {
        if (document.hidden) {
          this.deactivate();
        } else {
          this.syncState();
        }
      });
    }

    attach(videoEl) {
      if (!videoEl) return this;
      if (this.video !== videoEl) {
        this.video = videoEl;
        if (!videoEl.crossOrigin) {
          try {
            videoEl.crossOrigin = 'anonymous';
          } catch (e) {}
        }
      }
      this.syncState();
      return this;
    }

    canGlow() {
      if (document.hidden) return false;

      // Check current page mode
      const isRecommend = typeof state !== 'undefined' && state.currentView === 'recommend';
      const playerModal = document.getElementById('player-modal');
      const isModalOpen = playerModal && playerModal.style.display !== 'none';

      // Ambient glow works in recommend feed and during player modal playback
      if (!isRecommend && !isModalOpen) {
        return false;
      }

      let v = this.video;
      if (isModalOpen) {
        const mv = document.getElementById('main-video');
        if (mv) {
          v = mv;
          this.video = mv;
        }
      } else if (isRecommend) {
        const fv = typeof getFeedVideo === 'function'
          ? getFeedVideo()
          : (document.getElementById('feed-video-0') || document.querySelector('.feed-video'));
        if (fv) {
          v = fv;
          this.video = fv;
        }
      }

      if (!v) return false;
      if (v.paused || v.ended || v.seeking || v.readyState < 2) {
        return false;
      }

      return true;
    }

    syncState() {
      if (this.canGlow()) {
        this.activate();
      } else {
        this.deactivate();
      }
    }

    start() {
      this.syncState();
    }

    pause() {
      this.deactivate();
    }

    stop() {
      this.deactivate();
    }

    activate() {
      const playerModal = document.getElementById('player-modal');
      const isModalOpen = playerModal && playerModal.style.display !== 'none';
      if (isModalOpen) {
        document.body.classList.remove('feed-ambient-glowing');
        document.body.classList.add('player-ambient-glowing');
      } else {
        document.body.classList.remove('player-ambient-glowing');
        document.body.classList.add('feed-ambient-glowing');
      }

      document.documentElement.style.setProperty('--feed-ambient-active', '1');

      if (!this.isRunning) {
        this.isRunning = true;
        this.loop();
      }
    }

    deactivate() {
      this.stopLoop();
      document.body.classList.remove('feed-ambient-glowing', 'player-ambient-glowing');
      document.documentElement.style.setProperty('--feed-ambient-active', '0');
      document.documentElement.style.setProperty('--feed-ambient-left', 'transparent');
      document.documentElement.style.setProperty('--feed-ambient-top', 'transparent');

      // Reset to neutral dark baseline
      this.currentLeft = [20, 22, 32];
      this.currentTop = [20, 22, 32];

      if (window.nativeMediaNotify) {
        window.nativeMediaNotify('AmbientGlow', { active: false });
      }
    }

    stopLoop() {
      this.isRunning = false;
      if (this.rafId !== null) {
        cancelAnimationFrame(this.rafId);
        this.rafId = null;
      }
    }

    loop() {
      if (!this.isRunning) return;

      if (!this.canGlow()) {
        this.deactivate();
        return;
      }

      const v = this.video;
      const now = performance.now();
      if (now - this.lastSampleTime >= this.sampleIntervalMs) {
        this.lastSampleTime = now;
        this.sampleEdgeColors(v);
      }

      this.rafId = requestAnimationFrame(() => this.loop());
    }

    sampleEdgeColors(v) {
      if (!v || !v.videoWidth || !v.videoHeight) return;

      try {
        this.sampleCtx.drawImage(v, 0, 0, 16, 16);

        // 1. Left Edge Strip (x: 0..2, y: 0..15)
        const leftData = this.sampleCtx.getImageData(0, 0, 3, 16).data;
        // 2. Top Edge Strip (x: 0..15, y: 0..2)
        const topData = this.sampleCtx.getImageData(0, 0, 16, 3).data;

        const rawLeft = this.extractAverageColor(leftData);
        const rawTop = this.extractAverageColor(topData);

        this.targetLeft = this.boostVibrancy(rawLeft);
        this.targetTop = this.boostVibrancy(rawTop);

        // Temporal EMA smoothing for liquid organic color flow
        const alpha = 0.16;
        this.currentLeft = this.blendColors(this.currentLeft, this.targetLeft, alpha);
        this.currentTop = this.blendColors(this.currentTop, this.targetTop, alpha);

        // Inject subtle diffuse values strictly for liquid glass surfaces
        const root = document.documentElement;
        root.style.setProperty(
          '--feed-ambient-left',
          `rgba(${this.currentLeft[0]}, ${this.currentLeft[1]}, ${this.currentLeft[2]}, 0.38)`
        );
        root.style.setProperty(
          '--feed-ambient-top',
          `rgba(${this.currentTop[0]}, ${this.currentTop[1]}, ${this.currentTop[2]}, 0.42)`
        );

        if (window.nativeMediaNotify) {
          window.nativeMediaNotify('AmbientGlow', {
            active: true,
            left: this.currentLeft,
            top: this.currentTop
          });
        }
      } catch (err) {
        if (!this._warned) {
          console.warn('[ambient-glow] sample error:', err);
          this._warned = true;
        }
      }
    }

    extractAverageColor(data) {
      let rTotal = 0;
      let gTotal = 0;
      let bTotal = 0;
      let count = 0;

      const len = data.length;
      for (let i = 0; i < len; i += 4) {
        const r = data[i];
        const g = data[i + 1];
        const b = data[i + 2];

        // Skip pure black borders, letterboxing, or dark letter edges
        if (r + g + b < 22) continue;

        rTotal += r;
        gTotal += g;
        bTotal += b;
        count++;
      }

      if (count < 3) {
        return [24, 26, 36];
      }

      return [
        Math.round(rTotal / count),
        Math.round(gTotal / count),
        Math.round(bTotal / count)
      ];
    }

    boostVibrancy(rgb) {
      let [r, g, b] = rgb;
      const max = Math.max(r, g, b);
      const min = Math.min(r, g, b);
      const diff = max - min;

      // Enhance chroma slightly if color has tone
      if (diff > 10) {
        const factor = 1.3;
        const avg = (r + g + b) / 3;
        r = Math.min(255, Math.max(0, avg + (r - avg) * factor));
        g = Math.min(255, Math.max(0, avg + (g - avg) * factor));
        b = Math.min(255, Math.max(0, avg + (b - avg) * factor));
      }

      // Ensure minimum luminance against obsidian dark glass
      const lum = 0.299 * r + 0.587 * g + 0.114 * b;
      if (lum < 35) {
        const scale = 35 / Math.max(lum, 1);
        r = Math.min(255, r * scale);
        g = Math.min(255, g * scale);
        b = Math.min(255, b * scale);
      }

      return [Math.round(r), Math.round(g), Math.round(b)];
    }

    blendColors(curr, target, alpha) {
      return [
        Math.round(curr[0] + (target[0] - curr[0]) * alpha),
        Math.round(curr[1] + (target[1] - curr[1]) * alpha),
        Math.round(curr[2] + (target[2] - curr[2]) * alpha)
      ];
    }
  }

  window.ambientGlow = new AmbientGlowController();
})();
