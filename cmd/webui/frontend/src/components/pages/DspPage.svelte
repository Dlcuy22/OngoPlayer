<script>
  // DspPage.svelte hosts the DSP and audio processing controls.
  //
  // Purpose: Provides an interface for equalizer and signal processing settings.
  // Currently displays a placeholder status.

  import Icon from "../Icon.svelte";
</script>

<div class="dsp-page">
  <div class="dsp-container">
    <Icon name="sliders-horizontal" size={36} strokeWidth={1.5} />
    <h1>DSP & Equalizer</h1>
    <p>Configure equalizer, sample rate converters, and audio effects.</p>
    <div class="coming-soon">Coming soon</div>
  </div>
</div>

<style>
  .dsp-page {
    flex: 1;
    display: flex;
    align-items: center;
    justify-content: center;
    background-color: var(--bg);
  }
  .dsp-container {
    display: flex;
    flex-direction: column;
    align-items: center;
    text-align: center;
    max-width: 320px;
    padding: 24px;
    color: var(--text-dim);
  }
  h1 {
    font-size: 18px;
    margin: 16px 0 8px 0;
    color: var(--text);
    font-weight: 700;
  }
  p {
    font-size: 13px;
    line-height: 1.5;
    margin: 0 0 20px 0;
    color: var(--text-faint);
  }
  .coming-soon {
    font-size: 11px;
    padding: 4px 10px;
    background: var(--surface-3);
    border: 1px solid var(--border);
    border-radius: 4px;
    color: var(--text-faint);
    letter-spacing: 0.05em;
    text-transform: uppercase;
  }
</style>
