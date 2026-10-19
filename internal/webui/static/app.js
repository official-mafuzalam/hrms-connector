const words={
  en:{menu:"MENU",status:"Status",settings:"Settings",logs:"Logs",about:"About",localOnly:"Local management",overview:"OVERVIEW",statusSub:"Connection health and device activity",refresh:"Refresh",hrmsServer:"HRMS SERVER",lastSuccess:"Last successful contact",activity:"ACTIVITY",totalRequests:"Total requests",errors:"Errors",upstream503:"Upstream 503",devices5m:"Devices · 5 min",devices:"Devices",devicesSub:"Recently active terminals",ip:"IP ADDRESS",lastSeen:"LAST SEEN",requests:"REQUESTS",noDevices:"No devices have connected yet.",deviceSetup:"Set on the device",deviceSetupSub:"Configure the terminal to send attendance data to this PC",serverAddress:"Server Address",port:"Port",domainName:"Enable Domain Name",https:"HTTPS",off:"OFF",copy:"Copy",menuSteps:"Menu > Comm. > Cloud Server Setting",deviceSteps:"Enter the Server Address and Port above. Keep Domain Name and HTTPS turned OFF.",configure:"CONFIGURE",settingsSub:"Manage your HRMS connection and local security",connectionKey:"Connection Key",verify:"Verify",relayPort:"Device relay port",check:"Check",portHelp:"Changes apply without restarting the connector.",language:"Language",newPIN:"New UI PIN (optional)",pinHelp:"A PIN is required for every settings change when configured.",removePIN:"Remove existing PIN",advanced:"Advanced settings",allowedCIDRs:"Allowed device networks (CIDRs)",timeout:"Upstream timeout (seconds)",maxBody:"Maximum request body (KB)",proxy:"HTTP proxy (optional)",logLevel:"Log level",verifyTLS:"Verify HRMS TLS certificate",save:"Save settings",diagnostics:"DIAGNOSTICS",logsSub:"Most recent 300 relay requests",download:"Download log",product:"PRODUCT",aboutSubtitle:"Secure attendance relay",version:"Version",dataDirectory:"Data directory",connected:"Connected to HRMS",invalid_key:"Connection key is invalid",revoked:"Connection key was revoked",unreachable:"HRMS server is unreachable",update:"Update available",keyVerified:"Key verified",saved:"Settings saved",copied:"Copied",portAvailable:"Port is available",portBusy:"Port is in use",verifyFirst:"Verify a new connection key before saving.",noAddress:"No LAN address found",none:"None yet",last5:"Seen in last 5 minutes",checkFailed:"Could not check port",saveFailed:"Could not save settings"},
  bn:{menu:"মেনু",status:"অবস্থা",settings:"সেটিংস",logs:"লগ",about:"সম্পর্কে",localOnly:"স্থানীয় ব্যবস্থাপনা",overview:"সারসংক্ষেপ",statusSub:"সংযোগের অবস্থা ও ডিভাইস কার্যকলাপ",refresh:"রিফ্রেশ",hrmsServer:"এইচআরএমএস সার্ভার",lastSuccess:"সর্বশেষ সফল যোগাযোগ",activity:"কার্যকলাপ",totalRequests:"মোট অনুরোধ",errors:"ত্রুটি",upstream503:"সার্ভার ৫০৩",devices5m:"ডিভাইস · ৫ মিনিট",devices:"ডিভাইস",devicesSub:"সাম্প্রতিক সক্রিয় টার্মিনাল",ip:"আইপি ঠিকানা",lastSeen:"সর্বশেষ দেখা",requests:"অনুরোধ",noDevices:"এখনও কোনো ডিভাইস সংযুক্ত হয়নি।",deviceSetup:"ডিভাইসে সেট করুন",deviceSetupSub:"এই পিসিতে উপস্থিতির তথ্য পাঠাতে টার্মিনাল কনফিগার করুন",serverAddress:"সার্ভার ঠিকানা",port:"পোর্ট",domainName:"ডোমেইন নাম চালু",https:"এইচটিটিপিএস",off:"বন্ধ",copy:"কপি",menuSteps:"মেনু > Comm. > Cloud Server Setting",deviceSteps:"উপরের সার্ভার ঠিকানা ও পোর্ট লিখুন। ডোমেইন নাম এবং HTTPS বন্ধ রাখুন।",configure:"কনফিগার",settingsSub:"এইচআরএমএস সংযোগ ও স্থানীয় নিরাপত্তা",connectionKey:"সংযোগ কী",verify:"যাচাই",relayPort:"ডিভাইস রিলে পোর্ট",check:"পরীক্ষা",portHelp:"কানেক্টর পুনরায় চালু ছাড়াই পরিবর্তন কার্যকর হবে।",language:"ভাষা",newPIN:"নতুন UI পিন (ঐচ্ছিক)",pinHelp:"পিন সেট করলে প্রতিটি সেটিং পরিবর্তনে পিন লাগবে।",removePIN:"বর্তমান পিন সরান",advanced:"উন্নত সেটিংস",allowedCIDRs:"অনুমোদিত ডিভাইস নেটওয়ার্ক (CIDR)",timeout:"সার্ভার টাইমআউট (সেকেন্ড)",maxBody:"সর্বোচ্চ অনুরোধের আকার (KB)",proxy:"HTTP প্রক্সি (ঐচ্ছিক)",logLevel:"লগ স্তর",verifyTLS:"HRMS TLS সার্টিফিকেট যাচাই",save:"সেটিংস সংরক্ষণ",diagnostics:"ডায়াগনস্টিক",logsSub:"সাম্প্রতিক ৩০০টি রিলে অনুরোধ",download:"লগ ডাউনলোড",product:"পণ্য",aboutSubtitle:"নিরাপদ উপস্থিতি রিলে",version:"সংস্করণ",dataDirectory:"ডেটা ডিরেক্টরি",connected:"HRMS-এর সাথে সংযুক্ত",invalid_key:"সংযোগ কী সঠিক নয়",revoked:"সংযোগ কী বাতিল করা হয়েছে",unreachable:"HRMS সার্ভারে পৌঁছানো যাচ্ছে না",update:"আপডেট উপলভ্য",keyVerified:"কী যাচাই হয়েছে",saved:"সেটিংস সংরক্ষিত হয়েছে",copied:"কপি হয়েছে",portAvailable:"পোর্ট ব্যবহারযোগ্য",portBusy:"পোর্টটি ব্যবহৃত হচ্ছে",verifyFirst:"সংরক্ষণের আগে নতুন সংযোগ কী যাচাই করুন।",noAddress:"কোনো LAN ঠিকানা পাওয়া যায়নি",none:"এখনও নেই",last5:"গত ৫ মিনিটে দেখা হয়েছে",checkFailed:"পোর্ট পরীক্ষা করা যায়নি",saveFailed:"সেটিংস সংরক্ষণ করা যায়নি"}
};
Object.assign(words.en,{setup:"Device Setup",setupSub:"Follow these steps to connect a time attendance device.",setPIN:"Set a PIN",keyStatus:"HRMS connection",deviceValues:"Enter these values on the device",serverMode:"Server Mode",proxySetting:"Proxy",suggestedNetwork:"Suggested device network settings",suggestedNetworkNote:"Suggestions are best effort. Check with your network administrator before using them.",suggestedIPs:"Possible unused IPs",subnetMask:"Subnet mask",gateway:"Gateway",dns:"DNS",deviceMenuPath:"Device menu and reminders",deviceReminder:"Also set the correct date/time and set Punch State to Auto.",deviceConnection:"Device connection",waitingDevice:"Waiting for a device…",deviceConnected:"Device connected",print:"Print setup",uiPort:"Local UI port",uiPortFixed:"Fixed after installation.",relayPortWarning:"Changing this port requires updating every device.",relayUnavailable:"Relay is unavailable",noNetworkDetails:"Network details are unavailable",chooseAdapter:"Choose the network adapter",saveChoice:"Use adapter",needsChoice:"Needs adapter choice"});
Object.assign(words.bn,{setup:"ডিভাইস সেটআপ",setupSub:"সময় উপস্থিতি ডিভাইস সংযোগ করতে এই ধাপগুলো অনুসরণ করুন।",setPIN:"পিন সেট করুন",keyStatus:"এইচআরএমএস সংযোগ",deviceValues:"ডিভাইসে এই মানগুলো লিখুন",serverMode:"সার্ভার মোড",proxySetting:"প্রক্সি",suggestedNetwork:"ডিভাইসের জন্য প্রস্তাবিত নেটওয়ার্ক সেটিংস",suggestedNetworkNote:"এই পরামর্শগুলো সম্ভাব্য মাত্র। ব্যবহারের আগে নেটওয়ার্ক প্রশাসকের সাথে নিশ্চিত করুন।",suggestedIPs:"সম্ভাব্য অব্যবহৃত আইপি",subnetMask:"সাবনেট মাস্ক",gateway:"গেটওয়ে",dns:"ডিএনএস",deviceMenuPath:"ডিভাইস মেনু ও অনুস্মারক",deviceReminder:"সঠিক তারিখ/সময় সেট করুন এবং Punch State Auto নির্বাচন করুন।",deviceConnection:"ডিভাইস সংযোগ",waitingDevice:"ডিভাইসের জন্য অপেক্ষা করা হচ্ছে…",deviceConnected:"ডিভাইস সংযুক্ত হয়েছে",print:"সেটআপ প্রিন্ট করুন",uiPort:"স্থানীয় UI পোর্ট",uiPortFixed:"ইনস্টলের পর অপরিবর্তনীয়।",relayPortWarning:"এই পোর্ট বদলালে প্রতিটি ডিভাইসেও পোর্ট বদলাতে হবে।",relayUnavailable:"রিলে উপলভ্য নয়",noNetworkDetails:"নেটওয়ার্কের তথ্য পাওয়া যায়নি",chooseAdapter:"নেটওয়ার্ক অ্যাডাপ্টার নির্বাচন করুন",saveChoice:"এই অ্যাডাপ্টার ব্যবহার করুন",needsChoice:"অ্যাডাপ্টার নির্বাচন প্রয়োজন"});
Object.assign(words.en,{details:"Details",networkAddress:"PC network address",networkAddressSub:"Keep the address the devices use from changing",fixed:"Fixed",automatic:"Automatic (may change)",unknown:"Unknown",fixIP:"Fix IP address",routerWarning:"Your router must not assign this address to another device. We recommend creating a DHCP reservation for this PC in the router settings.",networkUnavailable:"Network status unavailable",networkFixed:"The current address is now fixed without changing the IP.",networkAlreadyFixed:"This address is already fixed.",networkFixFailed:"Could not fix the network address"});
Object.assign(words.bn,{details:"বিস্তারিত",networkAddress:"এই পিসির নেটওয়ার্ক ঠিকানা",networkAddressSub:"ডিভাইসের ব্যবহৃত ঠিকানা পরিবর্তন হওয়া থেকে রক্ষা করুন",fixed:"নির্দিষ্ট",automatic:"স্বয়ংক্রিয় (পরিবর্তন হতে পারে)",unknown:"অজানা",fixIP:"আইপি ঠিকানা নির্দিষ্ট করুন",routerWarning:"আপনার রাউটার যেন এই ঠিকানাটি অন্য ডিভাইসকে না দেয়। রাউটারে এই পিসির জন্য DHCP reservation করার পরামর্শ দিচ্ছি।",networkUnavailable:"নেটওয়ার্ক অবস্থা পাওয়া যায়নি",networkFixed:"আইপি পরিবর্তন না করে বর্তমান ঠিকানাটি নির্দিষ্ট করা হয়েছে।",networkAlreadyFixed:"এই ঠিকানাটি ইতিমধ্যে নির্দিষ্ট।",networkFixFailed:"নেটওয়ার্ক ঠিকানা নির্দিষ্ট করা যায়নি"});
let lang="bn",csrf="",statusData=null,verifiedKey="",uiPin="";
const t=(key)=>words[lang][key]||words.en[key]||key;
const $=(id)=>document.getElementById(id);

async function api(path,options={}){
  const headers={...(options.headers||{})};
  if(options.method==="POST"){
    headers["Content-Type"]="application/json";
    headers["X-CSRF-Token"]=csrf;
    if(uiPin)headers["X-UI-PIN"]=uiPin;
  }
  const response=await fetch(path,{...options,headers,credentials:"same-origin"});
  if(response.status===401&&options.method==="POST"){
    uiPin=prompt("Enter UI PIN")||"";
    if(uiPin)return api(path,options);
  }
  if(!response.ok){
    const text=await response.text();
    let failure;
    try{
      failure=JSON.parse(text);
    }catch{}
    const message=failure?.[`message_${lang}`]||failure?.message;
    if(message)throw new Error(`${message}${failure.details?`\n${t("details")}: ${failure.details}`:""}`);
    throw new Error(text.trim()||`Request failed (${response.status})`);
  }
  return response.json();
}

function toast(message){
  $("toast").textContent=message;
  $("toast").classList.add("show");
  setTimeout(()=>$("toast").classList.remove("show"),1800);
}

function setLanguage(value){
  lang=value;
  document.documentElement.lang=value;
  document.querySelectorAll("[data-t]").forEach(el=>el.textContent=t(el.dataset.t));
  $("language").textContent=value==="bn"?"English":"বাংলা";
  if(statusData)renderStatus(statusData);
  if($("page-setup")&&!$("page-setup").classList.contains("hidden"))loadSetup();
}

function showPage(page){
  document.querySelectorAll(".page").forEach(el=>el.classList.toggle("hidden",el.id!==`page-${page}`));
  document.querySelectorAll(".nav").forEach(el=>el.classList.toggle("active",el.dataset.page===page));
  if(page==="logs")loadLogs();
  if(page==="about")loadAbout();
  if(page==="settings")loadSettings();
  if(page==="setup")loadSetup();
}

async function loadStatus(){
  try{
    statusData=await api("/api/status");
    renderStatus(statusData);
  }catch(error){
    $("connection-title").textContent=t("unreachable");
    $("connection-detail").textContent=error.message;
  }
}

function versionIsNewer(latest,current){
  const parse=value=>String(value||"").replace(/^v/,"").split(/[.+-]/).slice(0,3).map(part=>Number.parseInt(part,10)||0);
  const a=parse(latest),b=parse(current);
  for(let i=0;i<3;i++)if(a[i]!==b[i])return a[i]>b[i];
  return false;
}

function renderStatus(data){
  const hb=data.heartbeat||{},stats=data.stats||{},state=hb.state||"unreachable";
  const banner=$("connection-banner");
  banner.className=`banner ${data.relay_listening===false?"red":state==="connected"?"green":state==="unreachable"?"amber":"red"}`;
  $("connection-title").textContent=data.relay_listening===false?t("relayUnavailable"):t(state);
  $("server-state").textContent=state.replace("_"," ");
  $("company").textContent=hb.company||"—";
  $("site").textContent=hb.site||"—";
  $("last-ok").textContent=hb.last_ok?new Date(hb.last_ok).toLocaleString():"—";
  $("total-requests").textContent=stats.requests||0;
  $("total-errors").textContent=stats.errors||0;
  $("upstream-errors").textContent=stats.upstream_503||0;
  $("version-top").textContent=`v${data.version}`;
  $("device-port").textContent=data.port;
  $("key-masked").textContent=data.key_masked?`Saved: ${data.key_masked}`:"";

  const devices=(data.devices||[]).slice().sort((a,b)=>new Date(b.last_seen)-new Date(a.last_seen));
  const active=devices.filter(device=>Date.now()-new Date(device.last_seen)<300000);
  $("active-devices").textContent=active.length;
  const errorDetail=hb.error?`${t(state)}\n${t("details")}: ${hb.error}\n`:"";
  const relayError=data.relay_listening===false?`${t("details")}: ${data.relay_listener_error||t("relayUnavailable")}\n`:"";
  $("connection-detail").textContent=`${relayError}${errorDetail}${active.length} ${t("last5")}`;
  const body=$("device-rows");
  body.replaceChildren();
  if(!devices.length){
    const row=body.insertRow(),cell=row.insertCell();
    cell.colSpan=4;cell.className="empty";cell.textContent=t("noDevices");
  }else{
    devices.forEach(device=>{
      const row=body.insertRow();
      [device.sn,device.ip,new Date(device.last_seen).toLocaleString(),device.count].forEach(value=>row.insertCell().textContent=value);
    });
  }
  const addresses=data.lan_addresses||[];
  const network=data.netcfg;
  const fixButton=$("fix-network");
  $("lan-address").textContent=(network&&network.ip)||addresses[0]||t("noAddress");
  $("other-addresses").textContent=addresses.filter(address=>address!==$("lan-address").textContent).join(", ");
  if(network){
    const choiceWrap=$("adapter-choice-wrap");
    choiceWrap.classList.toggle("hidden",!network.needs_choice);
    if(network.needs_choice){
      $("network-state").textContent=t("needsChoice");
      $("network-detail").textContent=network.note||t("chooseAdapter");
      const select=$("adapter-choice");select.replaceChildren();
      (network.options||[]).forEach(option=>{
        const item=document.createElement("option");item.value=option.interface;
        item.textContent=`${option.interface} · ${option.ip}/${option.prefix} · metric ${option.metric}`;
        select.append(item);
      });
    }else{
      const mode=network.dhcp?t("automatic"):t("fixed");
      $("network-state").textContent=`IP ${network.ip}: ${mode}`;
      $("network-detail").textContent=network.note||data.netcfg_error||`${network.interface}${network.wifi?` · Wi-Fi`:""}`;
    }
    fixButton.classList.toggle("hidden",!network.dhcp||network.netplan);
    fixButton.disabled=!!network.netplan||!!network.needs_choice;
  }else{
    $("network-state").textContent=t("networkUnavailable");
    $("network-detail").textContent=data.netcfg_error||"";
    fixButton.classList.add("hidden");
  }
  $("adapter-choice-wrap").classList.toggle("hidden",!network||!network.needs_choice);
  const update=$("update-notice");
  update.replaceChildren();
  if(hb.latest_version&&versionIsNewer(hb.latest_version,data.version)){
    update.append(`${t("update")}: ${hb.latest_version}`);
    try{
      const download=new URL(hb.download_url);
      if(download.protocol==="https:"){
        const link=document.createElement("a");
        link.href=download.href;link.textContent=` · ${t("download")}`;link.target="_blank";link.rel="noopener noreferrer";
        update.append(link);
      }
    }catch{}
    update.classList.remove("hidden");
  }else update.classList.add("hidden");
}

async function loadSettings(){
  try{
    const data=await api("/api/status");
    statusData=data;
    $("key-input").value="";
    $("key-input").placeholder=data.key_masked||"HRMS1.…";
    $("key-masked").textContent=data.key_masked?`Saved: ${data.key_masked}`:"";
    $("port-input").value=data.port;
    $("ui-port-input").value=data.ui_port;
    $("language-input").value=data.language;
    $("timeout-input").value=data.upstream_timeout;
    $("body-input").value=data.max_body_kb;
    $("log-input").value=data.log_level;
    $("tls-input").checked=data.tls_verify;
    $("proxy-input").value=data.http_proxy||"";
    $("cidrs-input").value=(data.allowed_cidrs||[]).join("\n");
  }catch(error){toast(error.message)}
}

async function loadSetup(){
  try{
    const data=await api("/api/setup");
    const hb=data.heartbeat||{};
    $("setup-company").textContent=hb.company||t(hb.state||"unreachable");
    $("setup-site").textContent=hb.site||"";
    $("setup-connection").textContent=hb.state==="connected"?t("connected"):t(hb.state||"unreachable");
    const addressSelect=$("setup-address");
    const addresses=data.lan_addresses||[];
    if(data.selected_address&&!addresses.includes(data.selected_address))addresses.unshift(data.selected_address);
    addressSelect.replaceChildren();
    addresses.forEach(address=>{
      const option=document.createElement("option");
      option.value=address;option.textContent=address;
      addressSelect.append(option);
    });
    if(data.selected_address)addressSelect.value=data.selected_address;
    $("setup-port").textContent=data.port;
    const network=data.network||{};
    $("setup-mask").textContent=network.prefix?prefixToMask(network.prefix):"—";
    $("setup-gateway").textContent=network.gateway||"—";
    $("setup-dns").textContent=(network.dns||[]).join(", ")||"—";
    $("setup-network-note").textContent=data.network_error||data.suggestion_note||t("noNetworkDetails");
    const candidates=$("setup-candidates");
    candidates.replaceChildren();
    (data.suggested_ips||[]).forEach((ip,index)=>{
      const row=document.createElement("div");row.className="copy-value";
      const value=document.createElement("strong");value.textContent=ip;value.id=`setup-candidate-${index}`;
      const button=document.createElement("button");button.className="copy";button.dataset.copy=value.id;button.textContent=t("copy");
      row.append(value,button);candidates.append(row);
    });
    if(!candidates.children.length)candidates.textContent="—";
    const device=data.latest_device;
    const live=$("device-live");
    const deviceConnected=!!device&&Date.now()-new Date(device.last_seen).getTime()<300000;
    live.classList.toggle("connected",deviceConnected);
    $("setup-live-status").textContent=deviceConnected?t("deviceConnected"):t("waitingDevice");
    $("setup-live-detail").textContent=device?`${device.sn} · ${device.ip} · ${new Date(device.last_seen).toLocaleString()}`:"";
    document.querySelectorAll("[data-copy]").forEach(button=>button.onclick=()=>copyValue(button.dataset.copy));
  }catch(error){$("setup-connection").textContent=error.message}
}

function prefixToMask(prefix){
  const value=prefix===0?0:(0xffffffff<<(32-prefix))>>>0;
  return [24,16,8,0].map(shift=>(value>>>shift)&255).join(".");
}

async function copyValue(id){
  const element=$(id);
  const value=element instanceof HTMLSelectElement?element.value:element.textContent;
  try{await navigator.clipboard.writeText(value);toast(t("copied"))}
  catch{toast("Clipboard unavailable")}
}

async function loadLogs(){
  try{
    const data=await api("/api/logs");
    $("log-lines").textContent=data.lines.join("\n")||t("none");
  }catch(error){$("log-lines").textContent=error.message}
}

async function loadAbout(){
  try{
    const data=await api("/api/about");
    $("about-version").textContent=data.version;
    $("about-dir").textContent=data.data_dir;
  }catch(error){$("about-dir").textContent=error.message}
}

document.querySelectorAll(".nav").forEach(button=>button.addEventListener("click",()=>showPage(button.dataset.page)));
$("language").addEventListener("click",()=>{setLanguage(lang==="bn"?"en":"bn");if(statusData)loadStatus()});
$("refresh").addEventListener("click",loadStatus);
document.querySelectorAll("[data-copy]").forEach(button=>button.addEventListener("click",()=>copyValue(button.dataset.copy)));
$("verify-key").addEventListener("click",async()=>{
  const value=$("key-input").value.trim();
  if(!value){$("verify-result").textContent=t("verifyFirst");return}
  try{
    const result=await api("/api/verify-key",{method:"POST",body:JSON.stringify({key:value})});
    verifiedKey=value;
    $("verify-result").textContent=`${t("keyVerified")}: ${result.company} · ${result.site}`;
  }catch(error){verifiedKey="";$("verify-result").textContent=error.message}
});
$("setup-pin").addEventListener("click",async()=>{
  const pin=prompt(t("setPIN"));
  if(pin===null)return;
  try{
    await api("/api/setup-pin",{method:"POST",body:JSON.stringify({pin})});
    uiPin=pin;toast(t("saved"));
  }catch(error){toast(error.message)}
});
$("save-adapter-choice").addEventListener("click",async()=>{
  try{
    await api("/api/select-adapter",{method:"POST",body:JSON.stringify({interface:$("adapter-choice").value})});
    await loadStatus();
  }catch(error){toast(error.message)}
});
$("check-port").addEventListener("click",async()=>{
  try{
    const result=await api("/api/check-port",{method:"POST",body:JSON.stringify({port:Number($("port-input").value)})});
    toast(result.available?t("portAvailable"):t("portBusy"));
  }catch{toast(t("checkFailed"))}
});
$("fix-network").addEventListener("click",async()=>{
  const button=$("fix-network");
  button.disabled=true;
  try{
    await api("/api/netfix",{method:"POST",body:"{}"});
    toast(t("networkFixed"));
    await loadStatus();
  }catch(error){
    toast(`${t("networkFixFailed")}: ${error.message}`);
  }finally{button.disabled=false}
});
$("settings-form").addEventListener("submit",async event=>{
  event.preventDefault();
  const newKey=$("key-input").value.trim();
  if(newKey&&newKey!==verifiedKey){$("save-result").textContent=t("verifyFirst");return}
  const payload={
    key:newKey,port:Number($("port-input").value),language:$("language-input").value,
    pin:$("pin-input").value,clear_pin:$("clear-pin").checked,
    allowed_cidrs:$("cidrs-input").value.split(/\r?\n/).map(value=>value.trim()).filter(Boolean),
    upstream_timeout:Number($("timeout-input").value),max_body_kb:Number($("body-input").value),
    tls_verify:$("tls-input").checked,http_proxy:$("proxy-input").value,log_level:$("log-input").value
  };
  try{
    await api("/api/settings",{method:"POST",body:JSON.stringify(payload)});
    $("key-input").value="";$("pin-input").value="";$("clear-pin").checked=false;verifiedKey="";
    $("save-result").textContent=t("saved");toast(t("saved"));setLanguage(payload.language);await loadStatus();
  }catch(error){$("save-result").textContent=error.message||t("saveFailed")}
});

(async()=>{
  try{
    const response=await fetch("/api/session",{credentials:"same-origin"});
    if(!response.ok)throw new Error("Could not start local session");
    csrf=(await response.json()).csrf;
    await loadStatus();setLanguage(lang);
    const startPage=location.pathname==="/setup"?"setup":"status";
    showPage(startPage);
    setInterval(loadStatus,10000);
    setInterval(()=>{if(!$("page-setup").classList.contains("hidden"))loadSetup()},3000);
  }catch(error){$("connection-title").textContent=error.message}
})();
