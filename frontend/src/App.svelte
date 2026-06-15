<script lang="ts">
  import { Service } from '../bindings/github.com/patyhank/linzone-hub/internal/app/index.js';

  type DeviceSummary = { index: number; path: string; model: string; product: string; manufacturer: string; serial: string; vendorId: number; productId: number; usagePage: number; usage: number; interface: number; kind: 'headset' | 'buds' | 'mouse' | 'keyboard' | string; isBuds: boolean };
  type BatteryCell = { percent: number; status: number };
  type VolumeState = { mute: number; raw: number; percent: number };
  type AmbientState = { mode: number; ambientRaw: number; ambientPercent: number; voiceFocus: boolean };
  type SimpleState = { eventId: number; value?: number | null; raw: number[] };
  type DeviceState = {
    device: DeviceSummary;
    battery: { headset?: BatteryCell | null; left?: BatteryCell | null; right?: BatteryCell | null; case?: BatteryCell | null };
    headphoneVolume?: VolumeState | null; micVolume?: VolumeState | null; ambient?: AmbientState | null; gameChatMix?: SimpleState | null; sidetone?: VolumeState | null;
    surround?: SimpleState | null; btStatus?: SimpleState | null; btSoundQuality?: SimpleState | null; btStartupMode?: SimpleState | null; autoPowerOff?: SimpleState | null; ncStartupMode?: SimpleState | null; connectionMode?: SimpleState | null; assignable?: SimpleState | null;
    warnings: string[];
  };
  type Profile = { id: number; name: string; preset: string; axis: string; eq: number[]; controls: Record<string, number | boolean> };
  type ModelIconMap = { products?: Record<string, { marketing_name: string; icon_file: string }>; pid_map?: Record<string, string> };

  const eqBands = ['31.5', '63', '125', '250', '500', '1k', '2k', '4k', '8k', '16k'];
  const eqPresets = ['FLAT', 'BASS BOOST', 'MUSIC / VIDEO', 'FPS', 'CUSTOM'];
  const eqAxes = ['STANDARD', 'IMMERSION', 'VICTORY'];
  const tabs = ['Live', 'Profiles', 'Bluetooth', 'Buttons', 'Spatial'] as const;
  const touchActions = [
    ['Mic mute', 1], ['Volume up', 2], ['Volume down', 3], ['NC / Ambient / Off', 4], ['Game up', 5], ['Chat up', 6], ['Play / Pause', 7], ['Previous', 8], ['Next', 9], ['Quick attention', 10], ['No function', 255],
  ];
  const autoPower = [0, 5, 15, 30, 60, 180];

  let devices: DeviceSummary[] = $state([]);
  let selectedIndex = $state(0);
  let deviceState: DeviceState | null = $state(null);
  let modelMap: ModelIconMap = $state({});
  let activeTab: (typeof tabs)[number] = $state('Live');
  let loading = $state(false);
  let message = $state('Ready');
  let error = $state('');

  let headphoneVolume = $state(50), micVolume = $state(50), sidetone = $state(20), gameChatMix = $state(50);
  let ambientMode = $state(2), ambientLevel = $state(50), voiceFocus = $state(false);
  let surroundEnabled = $state(false), ncStartupMode = $state(0), btQuality = $state(0), btStartup = $state(0), autoPowerOff = $state(30), connectionMode = $state(0), incomingPermission = $state(true);
  let assignSlot = $state(0), assignAction = $state(1);
  let profileName = $state('Default headset profile'), selectedPreset = $state('FLAT'), selectedAxis = $state('STANDARD'), eqValues = $state(eqBands.map(() => 0));
  let profiles: Profile[] = $state([{ id: 1, name: 'Default headset profile', preset: 'FLAT', axis: 'STANDARD', eq: eqBands.map(() => 0), controls: {} }]);
  let liveTimers: Record<string, number> = {};
  let stateRequestId = 0;

  function hex(value: number): string { return `0x${value.toString(16).toUpperCase().padStart(4, '0')}`; }
  function batteryText(cell?: BatteryCell | null): string { return cell ? `${cell.percent}%` : 'N/A'; }
  function percentText(value?: number | null): string { return value === undefined || value === null ? 'N/A' : `${value}%`; }
  function stateText(state?: SimpleState | null): string { return state?.value === undefined || state?.value === null ? 'N/A' : String(state.value); }
  function activeDevice(): DeviceSummary | null { return devices.find((d) => d.index === selectedIndex) ?? null; }
  function hasPermissionWarning(state?: DeviceState | null): boolean {
    return Boolean(state?.warnings?.some((warning) => /permission|denied|拒絕|權限|hidraw/i.test(warning)));
  }
  function batterySummary(state?: DeviceState | null): string {
    if (!state) return 'Battery N/A';
    if (state.device.isBuds) return `L ${batteryText(state.battery.left)} · R ${batteryText(state.battery.right)} · Case ${batteryText(state.battery.case)}`;
    return `Battery ${batteryText(state.battery.headset)}`;
  }
  function clampPercent(value: number): number { return Math.max(0, Math.min(100, Number(value) || 0)); }
  function headphoneVolumeMax(): number { return activeDevice()?.isBuds ? 30 : 100; }
  function headphoneVolumeUnit(): string { return activeDevice()?.isBuds ? ' / 30' : '%'; }
  function clampHeadphoneVolume(value: number): number { return Math.max(0, Math.min(headphoneVolumeMax(), Number(value) || 0)); }

  function deviceIcon(device: DeviceSummary): string {
    const pidKey = `0x${device.productId.toString(16).toUpperCase().padStart(4, '0')}`;
    const modelKey = modelMap.pid_map?.[pidKey] ?? modelMap.pid_map?.[pidKey.toLowerCase()];
    const iconFile = modelKey ? modelMap.products?.[modelKey]?.icon_file : undefined;
    return `/inzone-assets/${iconFile ?? (device.isBuds ? 'raw/resources/menu_buds.png' : 'raw/resources/menu_headset.png')}`;
  }

  function syncControls(next: DeviceState | null) {
    if (!next) return;
    if (next.headphoneVolume) headphoneVolume = clampHeadphoneVolume(next.headphoneVolume.percent);
    if (next.micVolume) micVolume = clampPercent(next.micVolume.percent);
    if (next.sidetone) sidetone = clampPercent(next.sidetone.percent);
    if (next.gameChatMix?.value !== undefined && next.gameChatMix?.value !== null) gameChatMix = next.gameChatMix.value;
    if (next.ambient) { ambientMode = next.ambient.mode; ambientLevel = clampPercent(next.ambient.ambientPercent); voiceFocus = next.ambient.voiceFocus; }
    if (next.surround?.value !== undefined && next.surround?.value !== null) surroundEnabled = next.surround.value !== 0;
    if (next.ncStartupMode?.value !== undefined && next.ncStartupMode?.value !== null) ncStartupMode = next.ncStartupMode.value;
    if (next.btSoundQuality?.value !== undefined && next.btSoundQuality?.value !== null) btQuality = next.btSoundQuality.value;
    if (next.btStartupMode?.value !== undefined && next.btStartupMode?.value !== null) btStartup = next.btStartupMode.value;
    if (next.autoPowerOff?.value !== undefined && next.autoPowerOff?.value !== null) autoPowerOff = next.autoPowerOff.value;
    if (next.connectionMode?.value !== undefined && next.connectionMode?.value !== null) connectionMode = next.connectionMode.value;
  }

  function resetLiveControls() {
    for (const key of Object.keys(liveTimers)) window.clearTimeout(liveTimers[key]);
    liveTimers = {};
    deviceState = null;
    headphoneVolume = 0; micVolume = 0; sidetone = 0; gameChatMix = 50;
    ambientMode = 0; ambientLevel = 0; voiceFocus = false;
    surroundEnabled = false; ncStartupMode = 0; btQuality = 0; btStartup = 0; autoPowerOff = 30; connectionMode = 0;
  }

  async function selectDevice(index: number) {
    if (selectedIndex === index && deviceState) return;
    selectedIndex = index;
    resetLiveControls();
    message = '讀取裝置狀態...';
    await refreshState();
  }

  async function refreshDevices() {
    loading = true; error = '';
    try {
      devices = await Service.ListDevices();
      if (!devices.length) { deviceState = null; message = '未偵測到支援的 INZONE 裝置。請確認 USB/HID 權限與 udev rule。'; return; }
      if (!devices.some((d) => d.index === selectedIndex)) selectedIndex = devices[0].index;
      await refreshState();
    } catch (err) { error = String(err); } finally { loading = false; }
  }

  async function refreshState() {
    if (!devices.length) return;
    const requestId = ++stateRequestId;
    deviceState = null;
    loading = true; error = '';
    try {
      const nextState = await Service.GetDeviceState(selectedIndex);
      if (requestId !== stateRequestId) return;
      deviceState = nextState; syncControls(nextState); message = `已讀取 ${nextState?.device.model ?? 'device'}`;
    } catch (err) { error = String(err); } finally { loading = false; }
  }

  async function applyControl(label: string, action: () => Promise<void>, refresh = true) {
    loading = true; error = '';
    try { await action(); message = `${label} 已送出`; if (refresh) await refreshState(); } catch (err) { error = String(err); } finally { loading = false; }
  }

  async function repairUdevPermissions() {
    loading = true; error = ''; message = '等待系統授權以修復 udev 權限...';
    try {
      await Service.RepairUdevPermissions();
      message = 'udev 權限已修復，請重新插拔裝置或重新掃描。';
      await refreshDevices();
    } catch (err) {
      error = String(err);
    } finally {
      loading = false;
    }
  }

  function liveApply(key: string, label: string, action: () => Promise<void>) {
    window.clearTimeout(liveTimers[key]);
    liveTimers[key] = window.setTimeout(() => {
      void applyControl(label, action, false);
    }, 180);
  }

  function inputNumber(event: Event): number {
    return Number((event.currentTarget as HTMLInputElement).value);
  }

  function applyAmbientLive() {
    liveApply('ambient', 'Ambient/NC', () => Service.SetAmbient(selectedIndex, Number(ambientMode), Number(ambientLevel), voiceFocus));
  }

  function applySpatial() {
    void applyControl('Surround', () => Service.SetSurround(selectedIndex, surroundEnabled));
  }

  function saveProfile() {
    const controls = { headphoneVolume, micVolume, sidetone, gameChatMix, ambientMode, ambientLevel, voiceFocus, surroundEnabled, ncStartupMode, btQuality, btStartup, autoPowerOff, connectionMode };
    const next = { id: Date.now(), name: profileName, preset: selectedPreset, axis: selectedAxis, eq: [...eqValues], controls };
    profiles = [next, ...profiles.filter((p) => p.name !== profileName)];
    localStorage.setItem('linzone.profiles', JSON.stringify(profiles));
    message = `已儲存 profile：${profileName}`;
  }

  function loadProfile(profile: Profile) {
    profileName = profile.name; selectedPreset = profile.preset; selectedAxis = profile.axis; eqValues = [...profile.eq];
    headphoneVolume = Number(profile.controls.headphoneVolume ?? headphoneVolume);
    micVolume = Number(profile.controls.micVolume ?? micVolume);
    sidetone = Number(profile.controls.sidetone ?? sidetone);
    gameChatMix = Number(profile.controls.gameChatMix ?? gameChatMix);
    ambientMode = Number(profile.controls.ambientMode ?? ambientMode);
    ambientLevel = Number(profile.controls.ambientLevel ?? ambientLevel);
    voiceFocus = Boolean(profile.controls.voiceFocus ?? voiceFocus);
    surroundEnabled = Boolean(profile.controls.surroundEnabled ?? surroundEnabled);
    message = `已載入 profile：${profile.name}`;
  }

  function exportProfiles() { message = JSON.stringify(profiles); }

  $effect(() => {
    fetch('/inzone-assets/model_icon_map.json').then((r) => r.json()).then((value) => (modelMap = value)).catch(() => undefined);
    const saved = localStorage.getItem('linzone.profiles');
    if (saved) profiles = JSON.parse(saved);
    refreshDevices();
  });
</script>

<main class="shell">
  <section class="topbar">
    <div class="brand"><img src="/inzone-assets/resources/app_notify_icon.png" alt="" /><div><h1>LINZONE Hub</h1><p>Unofficial Linux controller for INZONE headset features</p></div></div>
    <div class="actions"><button class="primary" onclick={refreshDevices} disabled={loading}>重新掃描</button><button onclick={refreshState} disabled={loading || devices.length === 0}>刷新狀態</button></div>
  </section>

  {#if error}<div class="banner error">{error}</div>{:else}<div class="banner">{message}</div>{/if}
  {#if hasPermissionWarning(deviceState)}
    <div class="banner warning">
      <span>偵測到 hidraw 權限不足，部分控制功能無法開啟裝置。</span>
      <button onclick={repairUdevPermissions} disabled={loading}>修復 udev 權限</button>
    </div>
  {/if}

  <div class="layout">
    <aside class="sidebar">
      <div class="panel-title"><h2>裝置</h2><span>{devices.length}</span></div>
      {#each devices as device}
        <button class:active={selectedIndex === device.index} class="device-card" onclick={() => selectDevice(device.index)}>
          <img src={deviceIcon(device)} alt="" /><span><strong>{device.model}</strong><small>{device.product || 'INZONE device'}</small></span>
        </button>
      {:else}<p class="muted">沒有耳機/耳塞。</p>{/each}
      <div class="mini-status"><b>{activeDevice()?.model ?? 'No device'}</b><span>{batterySummary(deviceState)}</span></div>
    </aside>

    <section class="workspace">
      <nav class="tabs">{#each tabs as tab}<button class:active={activeTab === tab} onclick={() => (activeTab = tab)}>{tab}</button>{/each}</nav>

      {#if activeTab === 'Live'}
        <article class="panel hero-device">
          <img src={activeDevice() ? deviceIcon(activeDevice()!) : '/inzone-assets/raw/resources/menu_headset.png'} alt="" />
          <div><h2>{activeDevice()?.model ?? 'INZONE device'}</h2><p>{deviceState?.device.product || '已連線'}</p></div>
          <div class="state-pills"><span>{batterySummary(deviceState)}</span><span>BT {stateText(deviceState?.btStatus)}</span><span>Mix {stateText(deviceState?.gameChatMix)}</span><span>Sidetone {percentText(deviceState?.sidetone?.percent)}</span></div>
        </article>

        <article class="panel controls">
          <div class="panel-title"><h2>音訊控制</h2><span>即時套用</span></div>
          <div class="control-grid live-only">
            <label>耳機音量 <b>{headphoneVolume}{headphoneVolumeUnit()}</b><input type="range" min="0" max={headphoneVolumeMax()} bind:value={headphoneVolume} oninput={(event) => { const value = inputNumber(event); headphoneVolume = value; liveApply('headphoneVolume', '耳機音量', () => Service.SetHeadphoneVolume(selectedIndex, value)); }} /></label>
            <label>麥克風音量 <b>{micVolume}%</b><input type="range" min="0" max="100" bind:value={micVolume} oninput={(event) => { const value = inputNumber(event); micVolume = value; liveApply('micVolume', '麥克風音量', () => Service.SetMicVolume(selectedIndex, value)); }} /></label>
            <label>Sidetone <b>{sidetone}%</b><input type="range" min="0" max="100" bind:value={sidetone} oninput={(event) => { const value = inputNumber(event); sidetone = value; liveApply('sidetone', 'Sidetone', () => Service.SetSidetone(selectedIndex, value)); }} /></label>
            <label>Game / Chat mix <b>{gameChatMix}%</b><input type="range" min="0" max="100" bind:value={gameChatMix} oninput={(event) => { const value = inputNumber(event); gameChatMix = value; liveApply('gameChatMix', 'Game / Chat mix', () => Service.SetGameChatMix(selectedIndex, value)); }} /></label>
          </div>
        </article>

        <article class="panel">
          <div class="panel-title"><h2>Noise Canceling / Ambient</h2><span>環境音控制</span></div>
          <div class="row four">
            <label>模式<select bind:value={ambientMode} onchange={applyAmbientLive}><option value={0}>Off</option><option value={1}>Noise canceling</option><option value={2}>Ambient</option><option value={3}>Custom</option></select></label>
            <label>Ambient level <b>{ambientLevel}%</b><input type="range" min="0" max="100" bind:value={ambientLevel} oninput={(event) => { ambientLevel = inputNumber(event); applyAmbientLive(); }} /></label>
            <label class="check"><input type="checkbox" bind:checked={voiceFocus} onchange={applyAmbientLive} /> Voice focus</label>
          </div>
        </article>
      {/if}

      {#if activeTab === 'Profiles'}
        <article class="panel">
          <div class="panel-title"><h2>Profiles / 10-band EQ</h2><span>設定檔</span></div>
          <div class="row four"><input bind:value={profileName} aria-label="profile name" /><select bind:value={selectedPreset}>{#each eqPresets as preset}<option>{preset}</option>{/each}</select><select bind:value={selectedAxis}>{#each eqAxes as axis}<option>{axis}</option>{/each}</select><button class="primary" onclick={saveProfile}>儲存</button></div>
          <div class="eq-grid">{#each eqBands as band, index}<label><span>{band}Hz</span><input type="range" min="-10" max="10" bind:value={eqValues[index]} /><b>{eqValues[index]} dB</b></label>{/each}</div>
          <div class="profile-list">{#each profiles as profile}<button onclick={() => loadProfile(profile)}><strong>{profile.name}</strong><span>{profile.preset} · {profile.axis}</span></button>{/each}<button onclick={exportProfiles}>匯出 JSON</button></div>
        </article>
      {/if}

      {#if activeTab === 'Bluetooth'}
        <article class="panel">
          <div class="panel-title"><h2>Bluetooth controls</h2><span>連線設定</span></div>
          <div class="row four">
            <label>音質優先<select bind:value={btQuality}><option value={0}>Standard</option><option value={1}>Sound quality</option><option value={2}>Stable connection</option></select></label>
            <label>BT startup<select bind:value={btStartup}><option value={0}>Off</option><option value={1}>Last state</option><option value={2}>On</option></select></label>
            <label>Auto power off<select bind:value={autoPowerOff}>{#each autoPower as minutes}<option value={minutes}>{minutes === 0 ? 'Never' : `${minutes} min`}</option>{/each}</select></label>
            <label>Connection mode<select bind:value={connectionMode}><option value={0}>PC / USB</option><option value={1}>Bluetooth</option><option value={2}>Auto</option><option value={3}>Dual</option></select></label>
          </div>
          <div class="button-row"><button onclick={() => applyControl('BT sound quality', () => Service.SetBTSoundQuality(selectedIndex, btQuality))}>套用音質</button><button onclick={() => applyControl('BT startup', () => Service.SetBTStartupMode(selectedIndex, btStartup))}>套用啟動</button><button onclick={() => applyControl('Auto power off', () => Service.SetAutoPowerOff(selectedIndex, autoPowerOff))}>套用自動關機</button><button onclick={() => applyControl('Connection mode', () => Service.SetConnectionDestinationMode(selectedIndex, connectionMode))}>套用連線</button></div>
        </article>
      {/if}

      {#if activeTab === 'Buttons'}
        <article class="panel">
          <div class="panel-title"><h2>Touch / button assignment</h2><span>按鍵功能</span></div>
          <div class="row three"><label>按鍵<input type="number" min="0" max="15" bind:value={assignSlot} /></label><label>功能<select bind:value={assignAction}>{#each touchActions as action}<option value={action[1]}>{action[0]}</option>{/each}</select></label><button class="primary" onclick={() => applyControl('Assignable action', () => Service.SetAssignableAction(selectedIndex, assignSlot, assignAction))}>指派</button></div>
        </article>
      {/if}

      {#if activeTab === 'Spatial'}
        <article class="panel spatial">
          <img src="/inzone-assets/raw/resources/surround_010_on.png" alt="" />
          <div><h2>HRTF / 360 Spatial</h2><p>360 Spatial / Surround</p><label class="check"><input type="checkbox" bind:checked={surroundEnabled} /> Surround enabled</label><button class="primary" onclick={applySpatial}>套用 Surround</button></div>
        </article>
      {/if}
    </section>
  </div>
</main>
