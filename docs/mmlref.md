# SiON MML Reference Manual (version 0.6.5)

*(c) keim +Si+ 2016*

---

## System operations
### System commands
```mml
#TITLE{title};
```
```mml
#SIGN{Fm}; fgab<cdef;  // Set key signitures as Fm (flats on b,e,a and d)
```
```mml
#REV; cdefgab>c;  // Same as "cdefgab<c"
```
```mml
#MACRO{static};  #A=cde;#B=Afg;o5B; #A=gfe;o4B; // Expand as "o5cdefg; o4cdefg"
```
```mml
#MACRO{dynamic}; #A=cde;#B=Afg;o5B; #A=gfe;o4B; // Expand as "o5cdefg; o4gfefg"
```
```mml
#VMODE{n88}; v15cv12cv9c; // v command on log scale
```
```mml
#TMODE{unit=100}; t10050cde; // t10050 means bpm=100.5
```
```mml
#TMODE{fps=60}; t30cde; // t30 means 30[frames/beat](bpm=120)
```
```mml
#QUANT16; q8cde; // Set maximum value of 'q' to 16
```
```mml
#FPS100;  // Set default value of '@fps' to 100
```
```mml
#END; Ignore text here.
```
| Statement | Range | Description |
| --- | --- | --- |
| #TITLE{...}; | (String) | #TITLE{title}; |
| #SIGN{...}; | [A-G][+#-b]?m? | #SIGN{Fm}; fgab<cdef; // Set key signitures as Fm (flats on b,e,a and d) |
| #REV{...}; | (octave\|volume) | #REV; cdefgab>c; // Same as "cdefgab<c" |
| #MACRO{...}; | (static\|dynamic) | #MACRO{dynamic}; #A=cde;#B=Afg;o5B; #A=gfe;o4B; // Expand as "o5cdefg; o4gfefg" |
| #VMODE{...}; | (n88\|mdx\|mck\|tss\|%x\|%v) | #VMODE{n88}; v15cv12cv9c; // v command on log scale |
| #TMODE{...}; | (unit\|fps\|timerb)=?n | #TMODE{fps=60}; t30cde; // t30 means 30[frames/beat](bpm=120) |
| #QUANTn; | 1 - (8) | #QUANT16; q8cde; // Set maximum value of 'q' to 16 |
| #FPSn; | 1 - 1000 (60) | #FPS100; // Set default value of '@fps' to 100 |
| #END; |  | #END; Ignore text here. |
**The format of '#SIGN{...};' system command**
| C, Am | (no key signifures) |
| --- | --- |
| G, Em | f+ |
| D, Bm | f+, c+ |
| A, F+m, F#m | f+, c+, g+ |
| E, C+m, C#m | f+, c+, g+, d+ |
| B, G+m, G#m | f+, c+, g+, d+, a+ |
| F+, F#, D+m, D#m | f+, c+, g+, d+, a+, e+ |
| C+, C#, A+m, A#m | f+, c+, g+, d+, a+, e+, b+ |
| F, Dm | b- |
| B-, Bb, Gm | b-, e- |
| E-, Eb, Cm | b-, e-, a- |
| A-, Ab, Fm | b-, e-, a-, d- |
| D-, Db, B-m, Bbm | b-, e-, a-, d-, g- |
| G-, Gb, E-m, Ebm | b-, e-, a-, d-, g-, c- |
| C-, Cb, A-m, Abm | b-, e-, a-, d-, g-, c-, f- |
Or, specify the key signitures spareted by comma.
#SIGN{f+,g-,a-,b-};  // Whole tone scale
### Macro definitions
```mml
#A=cde;   l8AAgedAd;    // Expand as "l8cdecdegedcded"
```
```mml
#A-C=cde; l8ABgedCd;    // Expand as "l8cdecdegedcded"
```
```mml
#A=cde; #B=efg; #AB+=fg; l8AB;    // Expand as "l8cdefgefgfg"
```
| Statement | Range | Description |
| --- | --- | --- |
| #[A-Z]=...; | (MML text) | #A-C=cde; l8ABgedCd; // Expand as "l8cdecdegedcded" |
| #[A-Z]+=...; | (MML text) | #A=cde; #B=efg; #AB+=fg; l8AB; // Expand as "l8cdefgefgfg" |
### Table definitions
```mml
#WAVB0{36454d4b41362f303639332309efd9cc362f220df2d9c8c3c6cbccc6bab0aeb7}; %4@0 cde;
```
```mml
#WAV0{(0,127)8,(127,-128)16,(-128,0)8}; %4@0 cde;
```
```mml
#WAVCOLOR0{08400f0f}; %4@0 cde;
```
```mml
#TABLE0{(64,0)5,(32,0)5,(16,0)5}; q8 na0 cde;
```
| Statement | Range | Description |
| --- | --- | --- |
| #WAVBn{...}; | n;0 - 255 | #WAVB0{36454d4b41362f303639332309efd9cc362f220df2d9c8c3c6cbccc6bab0aeb7}; %4@0 cde; |
| #WAVn{...}(formula...); | n;0 - 255 | #WAV0{(0,127)8,(127,-128)16,(-128,0)8}; %4@0 cde; |
| #WAVCOLOR/#WAVCn{...}; | n;0 - 255 | #WAVCOLOR0{08400f0f}; %4@0 cde; |
| #TABLEn{...}(formula...); | n;0 - 254 | #TABLE0{(64,0)5,(32,0)5,(16,0)5}; q8 na0 cde; |
**The format of #TABLEn{...}; and #WAVn{...};**
Entries are separated by comma.
ex) #TABLE0{0,2,4,6}
Entries are continue with "|" position.
ex) #TABLE0{0,2|4,6} // execute like (0,2,4,6,4,6,4,6...)
Format '[]n' makes repeating inner numbers.
例) #TABLE0{[0,1]3,2,3} (0,1,0,1,0,1,2,3)
Format '(a)n' means repeats 'a' n times.
ex) #TABLE0{(0)4} // same as #TABLE0{0,0,0,0}
Format '(a,b)n' means interpolates the range of [a,b) with n numbers. The expansion DOES NOT include last number 'b'.
ex) #TABLE0{(0,8)4} // same as #TABLE0{0,2,4,6}
Format '(a,b,c,...)n' means interpolates the list of [a,b,c,...) with n numbers.
例) #TABLE0{(0,6,3,9)9}  // same as #TABLE0{0,2,4,6,5,4,3,5,7}
The numbers are rounded when they are interpolated.
例) #TABLE0{(0,1,3)8} // same as #TABLE0{0,0,1,1,1,2,2,3}
You can repeat, magnify and offset the entries with writing '[repeat]*[magnify]+[offset]' after {...}.
Entries are rounded before calculations and the result is rounded again.
#TABLE0{0,1,2,3}3 // same as #TABLE0{0,0,0,1,1,1,2,2,2,3,3,3}
#TABLE0{0,1,2,3}*2 // same as #TABLE0{0,2,4,6}
#TABLE0{0,1,2,3}+2 // same as #TABLE0{2,3,4,5}
#TABLE0{0,1,2,3}3*2-2 // same as #TABLE0{-2,-2,-2,0,0,0,2,2,2,4,4,4}
#TABLE0{(0,3)6}*5 // same as #TABLE0{0,5,5,10,10,15}
#TABLE0{(0,6)6}*0.5 // same as #TABLE0{0,1,1,2,2,3}
### Parameters of FM sound module
```
#@0{
alg[0-15], fb[0-7], fbc[0-3],
(ws, ar, dr, sr, rr, sl, tl, ksr, ksl, mul, dt1, detune, ams, phase, fixedNote) x operator count
};
%6@0 cde;
```
```
#OPL@0{
alg[0-3], fb[0-7], 
(ws[0-7], ar[0-15], dr[0-15], rr[0-15], egt[0,1], sl[0-15], tl[0-63], ksr[0,1], ksl[0-3], mul[0-15], ams[0-3]) x operator count
};
%6@0 cde;
```
```
#OPM@0{
alg[0-7], fb[0-7], 
(ar[0-31], dr[0-31], sr[0-31], rr[0-15], sl[0-15], tl[0-127], ks[0-3], mul[0-15], dt1[0-7], dt2[0-3], ams[0-3]) x operator count
};
%6@0 cde;
```
```
#OPN@0{
alg[0-7], fb[0-7], 
(ar[0-31], dr[0-31], sr[0-31], rr[0-15], sl[0-15], tl[0-127], ks[0-3], mul[0-15], dt1[0-7], ams[0-3]) x operator count
};
%6@0 cde;
```
```
#OPX@0{
alg[0-15], fb[0-7], 
(ws[0-7], ar[0-31], dr[0-31], sr[0-31], rr[0-15], sl[0-15], tl[0-127], ks[0-3], mul[0-15], dt1[0-7], detune[], ams[0-3]) x operator count
};
%6@0 cde;
```
```
#MA@0{
alg[0-7], fb[0-7], 
(ws[0-31], ar[0-15], dr[0-15], sr[0-15], rr[0-15], sl[0-15], tl[0-63], ksr[0,1], ksl[0-3], mul[0-15], dt1[0-7], ams[0-3]) x operator count
};
%6@0 cde;
```
| Statement | Range | Description |
| --- | --- | --- |
| #@n{...}(sequence...); | n;0 - 255 | #@0{ alg[0-15], fb[0-7], fbc[0-3], (ws, ar, dr, sr, rr, sl, tl, ksr, ksl, mul, dt1, detune, ams, phase, fixedNote) x operator count }; %6@0 cde; |
| #OPL@n{...}(sequence...); | n;0 - 255 | #OPL@0{ alg[0-3], fb[0-7], (ws[0-7], ar[0-15], dr[0-15], rr[0-15], egt[0,1], sl[0-15], tl[0-63], ksr[0,1], ksl[0-3], mul[0-15], ams[0-3]) x operator count }; %6@0 cde; |
| #OPM@n{...}(sequence...); | n;0 - 255 | #OPM@0{ alg[0-7], fb[0-7], (ar[0-31], dr[0-31], sr[0-31], rr[0-15], sl[0-15], tl[0-127], ks[0-3], mul[0-15], dt1[0-7], dt2[0-3], ams[0-3]) x operator count }; %6@0 cde; |
| #OPN@n{...}(sequence...); | n;0 - 255 | #OPN@0{ alg[0-7], fb[0-7], (ar[0-31], dr[0-31], sr[0-31], rr[0-15], sl[0-15], tl[0-127], ks[0-3], mul[0-15], dt1[0-7], ams[0-3]) x operator count }; %6@0 cde; |
| #OPX@n{...}(sequence...); | n;0 - 255 | #OPX@0{ alg[0-15], fb[0-7], (ws[0-7], ar[0-31], dr[0-31], sr[0-31], rr[0-15], sl[0-15], tl[0-127], ks[0-3], mul[0-15], dt1[0-7], detune[], ams[0-3]) x operator count }; %6@0 cde; |
| #MA@n{...}(sequence...); | n;0 - 255 | #MA@0{ alg[0-7], fb[0-7], (ws[0-31], ar[0-15], dr[0-15], sr[0-15], rr[0-15], sl[0-15], tl[0-63], ksr[0,1], ksl[0-3], mul[0-15], dt1[0-7], ams[0-3]) x operator count }; %6@0 cde; |
**How to define and refer the FM module parameters.**
Define the FM module parameter as above, and write as '%6@[0-255]' in mml sequence to the load the FM sound.
In the definition of FM module parameters, letters other than digits are recognized as a separator (The comma, space and tab is recommended for a separator due to readability).
And the MML sequence after "{...}" block is executed when '%6@[0-255]' commands are called in MML sequences (see below example), however note commands(c,d,e,f,g,a,b,r) and tone commands(%,@) can not be included in it.
```mml
// Modify filter and quantize settings when change tone in a sequence.
#@0{ 8,  0,  0;
16, 40,  0, 34, 34,  0, 13, 1, 0, 1, 0,  0, 0, 0, 0;
1, 34, 28,  0,  0, 15, 35, 1, 0, 2, 0,  0, 0, 0, 0;
4, 63, 63,  0,  0,  0, 42, 1, 0, 5, 0,  0, 0, 0, 0;
0, 36,  0,  0, 34,  0,  0, 1, 0, 1, 0, -3, 0, 0, 0;
} @f96,2,32,72q4;
%6@0 cdefedc;
```
### Parameters of PCM sound module
| Statement | Range | Description |
| --- | --- | --- |
| #SAMPLERn{waveID, oneShotFlag, pan, channelCount, startPoint, endPoint, loopPoint}; | n;0 - 127 waveID;String oneShotFlag;0,1(0) pan;-64~64(0) channelCount;1,2(2) startPoint;Number(-1) endPoint;Number(-1) loopPoint;Number(-1) | Specify note number of this sample by n, and specify id string by waveID. When oneShotFlag is 1, the sound does not stop by gate time. The pan sets panning of this sample, The channelCount sets stereo or monoral. Refer the appendex about startPoint, endPoint and loopPoint. |
| Statement | Range | Description |
| --- | --- | --- |
| #PCMWAVEn{waveID, samplingNote, keyRangeFrom, keyRangeTo, channelCount, startPoint, endPoint, loopPoint}; | n;0 - 255 waveID;String samplingNote;0,1(0) keyRangeFrom;0~127(0) keyRangeTo;0~127(127) channelCount;1,2(2) startPoint;Number(-1) startPoint;Number(-1) loopPoint;Number(-1) | Specify PCM voice number by n, and specify id string by waveID. The samplingNote sets sampling point, keyRangeFrom and keyRangeTo set keyboard range of this wave. The channelCount sets stereo or monoral. Refer the appendex about startPoint, endPoint and loopPoint. |
| Statement | Range | Description |
| --- | --- | --- |
| #PCMVOICEn{volumeCenterNote, volumeKeyRange, volumeRange, panCenterNote, panKeyRange, panRange, ar, dr, sr, rr, sl}; | n;0 - 255 volumeCenterNote;0-127(4) volumeKeyRange;0-127(0) volumeRange;-128~128(0) panCenterNote;0-127(64) panKeyRange;0-127(0) panRange;-128~128 ar;0~63 dr;0~63 sr;0~63 rr;0~63 sl;0~15 | Specify PCM voice number by n, and changing rate of volume and panning by note number. And last 5 arguments set envelope. See appendex. |
**Setting of PCM samples playing position**
#SAMPLER and #PCMWAVE set playing position by startPoint, endPoint and loopPoint. These values are in sample count. SiON can detect and skip the silence at head and tail of mp3 file.
The startPoint sets starting position. Specify -1 to set starting point with skipping head silence.
The endPoint sets ending position. The negative value sets endPoint by the sample count from tail.
The loopPoint sets returning position by looping. Specify -1 for no-loop. Specify 0 (or less than startPoint) to set loopPoint to startPoint. The negative value sets loopPoint by the sample count from tail.
You cannot specify loopPoint less than startPoint. Use @ph command for that purpose.
**Parameters of #PCMVOICE**
#PCMVOICE sets the changing rate of volume and panning by note number. The negative value of volumeRange and panRange sets opsitte direction of slope.
### FM Channels Connecttion
```mml
#FM{B3(A)}; %5q8s63cde; %5q2cde;   // same as "@o1%5q8s63cde; @i3%5q2cde;"
```
| Statement | Range | Description |
| --- | --- | --- |
| #FM{...}; | (formula) | #FM{B3(A)}; %5q8s63cde; %5q2cde; // same as "@o1%5q8s63cde; @i3%5q2cde;" |
**Difference between emulated FM module(%6@n) and FM Channel Connecttion(#FM)**
In FM Channel Connecttion command,
you can play differnet sequence for each FM connected channels.
you can use the output after filtering by LP-Filter as FM modulator.
you can use the ring modulation.
you can apply differnet LFO and differnet table envelops for each FM connected channels.
On the other hand in emulated FM module,
you can connect feedback output from chosen operator (multi-operater feedback like an OPX).
of course you can play one sequence for all operators.
Light weighted (especialy with LFO).
You can connect emulated FM module channels by @o, @i and #FM commands. In this case, operator#0 of carrier channel is modulated by the output of modulator channel.
### Effector Settings
```mml
// Connect LPF->delay to Slot1．EffectSendLevel=32．
#EFFECT1{lf3000delay300,32,1}; @v64,32q0cder;
```
```mml
// Slot0 is the master effect. Effects final output.
#EFFECT0{autopan}; q0$cdefed;
```
```mml
// Slot1 output is connected to Slot2 by send_level of 64.
#EFFECT1{dist}@v128,64;#EFFECT2{delay}; %11@v0,32 cder;
```
| Statement | Range | Description |
| --- | --- | --- |
| #EFFECTn{...}; | (MML for effector) | // Slot1 output is connected to Slot2 by send_level of 64. #EFFECT1{dist}@v128,64;#EFFECT2{delay}; %11@v0,32 cder; |
**the MML for effector**
When you connect effectors by #EFFECT system command, you should use the MML for effectors (see following reference).
SiON MML allows to write "p", "@p", "@v" commands after "#EFFECT{}" (except for master slot), and you can specify the panning("p","@p") and mixing level("@v"/default at 128) for effector slot.
Otherwise, the arguments of "@v" commands specifies not only mixing level but also effect send level to following effector slots. For example, write "#EFFECT2{...}@v96,64,32;" in slot2, mixing level of slot2 is 96, send level to slot3 is 64 and send level to slot4 is 32.You can specify send level only to following effector slots.
| Statement | Range | Description |
| --- | --- | --- |
| eqlg,mg,hg,lf,hf | lg;Low gain[%](100) mg;Middle gain[%](100) hg;High gain[%](100) lf;Low freq.[Hz](800) hf;High freq.[Hz](5000) | 3Band EQualizer |
| wsdist,level | dist;Distortion[](50) level;Output level[%](100) | Wave Shaper |
| delaytime,fb,cross,wet | time;Delay time[ms](200) fb;Feedback[%](25) cross;Stereo channel crossing(0) wet;wet level[%](100) | DELAY |
| reverbdly1,dly2,fb,wet | dly1;Long delay time[%](70) dly1;Short delay time[%](40) fb;Feedback[%](80) wet;wet level[%](100) | REVERB |
| chorustime,fb,depth,wet | time;Delay time[ms](20) fb;Feedback[%](50) depth;Depth[](200) wet;wet level[%](100) | CHORUS |
| distpre,post,lpf,slope | pre;PreGain[dB](-60) post;PostGain[dB](-12) lpf;LPF Freq[Hz](2400) slope;LPF Slope[oct](1) | DISTortion |
| compthres,wnd,ar,rr,gain,level | thres;Threshold[%](70) wnd;Window Width[ms](50) ar;Attack[ms](20) rr;Release[ms](20) gain;Max gain[db](6) level;output level[%](50) | COMPressor |
| autopanfreq,depth | freq;Frequency[Hz](1) depth;Panning width[%](100) | AUTOPAN |
| stereowide,pan,phase | wide;enhancement[%](140) pan;Panning(0) phase;Phase invert(0) | STEREO enhancer |
| dsfreq, bits, ch | freq ; Freq.Shift(0) bits ; bit ratio(16) ch ; channel count(2) | Down Sampler. Freq.Shift=0 sets 44.1kHz, 1 sets 22.05kHz, 2 sets 11.02kHz ... |
| speakerhardness | hardness ; diaphragm hardness[%](10) | SPEAKER simulator |
| lffreq,band | freq;Frequency[Hz](800) band;band width[oct](1) | Low pass Filter |
| hffreq,band | freq;Frequency[Hz](5000) band;band width[oct](1) | High pass Filter |
| bffreq,band | freq;Frequency[Hz](3000) band;band width[oct](1) | Band pass Filter |
| nffreq,band | freq;Frequency[Hz](3000) band;band width[oct](1) | Notch (band stop) Filter |
| pffreq,band | freq;Frequency[Hz](3000) band;band width[oct](1) | Peaking Filter |
| affreq,band | freq;Frequency[Hz](3000) band;band width[oct](1) | All pass Filter |
| lbfreq,slope,gain | freq;Frequency[Hz](3000) slope;slope[oct](1) gain;gain[dB](6) | Low Booster |
| hbfreq,slope,gain | freq;Frequency[Hz](5500) slope;slope[oct](1) gain;gain[dB](6) | High Booster |
| nlfcut,res | cut;cutoff table index(255) res;resonance table index(255) | ENvelope controlable Low pass Filter |
| nhfcut,res | cut;cutoff table index(255) res;resonance table index(255) | ENvelope controlable High pass Filter |
| vowelout,f1,g1,f2,g2 | out;output[%](100) f1;1st freq.(800) g1;1st gain[dB](36) f2;2nd freq.(1300) g2;2nd gain[dB](24) | Vowel filter |
---

## MML commands

### Sequence control commands
```mml
t100cde t200cde;
```
```mml
l8 $cde;     // "cdecdecde..."(repeat infinitely)
```
```mml
l8 [cd|e]3;   // "cdecdecd"
```
```mml
@mask1 v1c;   // ignore "v1"
```
```mml
#A=cde; l8 AAA;         // expand as "cdecdecde"
```
```mml
#A=cde; l8 AA(2)A(-2);  // expand as "cdedef+>b-<cd"
```
```mml
// Comment
```
```mml
/* Comment */
```
| Statement | Range | Description |
| --- | --- | --- |
| tn | 1 - 511 (120) | t100cde t200cde; |
| $ |  | l8 $cde; // "cdecdecde..."(repeat infinitely) |
| [...\|...]n | 1 - 65535 (2) | l8 [cd\|e]3; // "cdecdecd" |
| @mask | 0 - 63 (0) | @mask1 v1c; // ignore "v1" |
| [A-Z] [A-Z](n) | -128 - 127 (0) | #A=cde; l8 AA(2)A(-2); // expand as "cdedef+>b-<cd" |
| // ... |  | // Comment |
| /* ... */ |  | /* Comment */ |
| ![n...!\|...!] | 1 - 256 (2) | [NOT RECOMMENDED] You can expand the loop when its comiping. You cannot nest this loop and the maximum value of repitation is 256. |
**the argument of '@mask' command.**
Specify a sum of below values you want to ignore.
1; (Volume related) x,v,@v,(,)
2; (Panning related) p,@p
4; (Key off timing) q,@q
8; (Operator settings) s,@,@al,@fb,@rr,@tl,@ml,@dt,@ph,@fx
16;(Table envelops) @@,na,np,nt,nf,_@@,_na,_np,_nt,_nf
32;(LFO modulations) ma,mp
### Pitch related commands
```mml
#SIGN{G}; cdef-gab<c  // natural on 'f'.
```
```mml
cder cder gedcdedr;
```
```mml
o4[co5c]  // plays 'o4c o5c o4c o5c'. the 'o' command is little strange at the entry of the loop.
```
```mml
o4c<c<c  // plays 'o4c o5c o6c'
```
```mml
o4[c<c]  // plays 'o4c o5c o4c o5c'. the '<' command is also strange at the entry of the loop.
```
```mml
k-2cde; k2cde;
```
```mml
cde; kt7cde;
```
```mml
c*<c;  // pitch intergradation for 1 octave
```
```mml
po6l8c&g&f&b&&ag&&fe&d&c4;  // portament on notes with slur.
```
| Statement | Range | Description |
| --- | --- | --- |
| [a-g][+#-]?n | 1 - 1920 (default value is specifyed by "l" command) | #SIGN{G}; cdef-gab<c // natural on 'f'. |
| rn | 1 - 1920 (default value is specifyed by "l" command) | cder cder gedcdedr; |
| on | 0 - 9 (5) | o4[co5c] // plays 'o4c o5c o4c o5c'. the 'o' command is little strange at the entry of the loop. |
| [<>]n | 1 - 9 (1) | o4[c<c] // plays 'o4c o5c o4c o5c'. the '<' command is also strange at the entry of the loop. |
| kn | -8192 - 8191 (0) | k-2cde; k2cde; |
| ktn | -128 - 127 (0) | cde; kt7cde; |
| * |  | c*<c; // pitch intergradation for 1 octave |
| pon | 0- | po6l8c&g&f&b&&ag&&fe&d&c4; // portament on notes with slur. |
| !@krn | -8192 - 8191 (0) | [NOT RECOMMENDED] (Key detune Relative)Relative key detune. |
| !@nsn | -128 - 127 (0) | [NOT RECOMMENDED] (Note Shift)Relative key transpose. |
### Length related commands
```mml
cder l8cder l16cder;
```
```mml
c4^16;  // play c with a length of 'quarter + 16th'
```
```mml
q0cr q4cr q8cr;
```
```mml
q8 @q48cr @q24cr @q0cr;  // same as "q0cr q4cr q8cr"
```
```mml
q8 @q,24 o5c2c2; q0 l8o4[8c]; // The timing of key-on delays 8th.
```
```mml
cd&e; // No key-off between "d" and "e"
```
```mml
c&c;   // No phase reset, so the boundary cannot be recognized.
```
```mml
#TABLE0{(0,128)90};na0 c&d&e;   // No envelop reset.
```
```mml
c&&c; // Phase reset at the beginning of 2nd note, so the boundary can be recognized.
```
```mml
#TABLE0{(0,128)90};na0 c&&d&&e;   // The envelop reset at the beginning of 2nd note.
```
| Statement | Range | Description |
| --- | --- | --- |
| ln | 1 - 1920 (4) | cder l8cder l16cder; |
| ^n | 1 - 1920 (default value is specifyed by "l" command) | c4^16; // play c with a length of 'quarter + 16th' |
| qn | 0 - 8 (6) | q0cr q4cr q8cr; |
| @qn1,n2 | 0 - 192 (0) | q8 @q,24 o5c2c2; q0 l8o4[8c]; // The timing of key-on delays 8th. |
| & |  | #TABLE0{(0,128)90};na0 c&d&e; // No envelop reset. |
| && |  | #TABLE0{(0,128)90};na0 c&&d&&e; // The envelop reset at the beginning of 2nd note. |
**The formula for key-on length**
The 'q' and '@q' commands are independent for the key-on length. the negative value of [key-on length] set 0.
[key-on length] = [length of note/rest] x [argument of 'q'] / 8 - ([1st argument of '@q'] + [2nd argument of '@q'])
### Volume related commands
```mml
%5 v4c v8c v12c v16c v20c v24c v28c v32c;
```
```mml
v1l8 c(c(c(c(c(c(c(c(c(c(c(c(c(c(c(c)4c)4c)4c)4c;
```
```mml
v8 x32c x64c x96c x128c v4 x32c x64c x96c x128c;  // "v" and "x" are independent. see below appendex.
```
```mml
%v3,1 v128c v96c v64c v32c;  // non-linear scale(dr=48dB)，v command max@128
```
```mml
%x1 x128c x120c x112c x104c;  // non-linear scale(dr=96dB)
```
```mml
@v64 v4c v8c v12c v16c @v32 v4c v8c v12c v16c;  // "@v" and "v" (and "x") are independent. see below appendex.
```
```mml
#EFFECT1{chorus}; @v64,32 cde;  // [effect send]=32 for effect slot #1 (chorus)
```
```mml
p0c p2c p4c p6c p8c;
```
```mml
@p-64c @p-32c @p0c @p32c @p64c; // same as above
```
| Statement | Range | Description |
| --- | --- | --- |
| vn | 0 - 32 (16) | %5 v4c v8c v12c v16c v20c v24c v28c v32c; |
| [()]n | 1 - 32 (1) | v1l8 c(c(c(c(c(c(c(c(c(c(c(c(c(c(c(c)4c)4c)4c)4c; |
| xn | 0 - 128 (128) | v8 x32c x64c x96c x128c v4 x32c x64c x96c x128c; // "v" and "x" are independent. see below appendex. |
| %vn1,n2 | n1=0 - 4(0) n2=0 - 7(4) | %v3,1 v128c v96c v64c v32c; // non-linear scale(dr=48dB)，v command max@128 |
| %xn | 0 - 4(0) | %x1 x128c x120c x112c x104c; // non-linear scale(dr=96dB) |
| @vn1,...n8 | 0 - 128 (n1=64/n2-n8=0) | #EFFECT1{chorus}; @v64,32 cde; // [effect send]=32 for effect slot #1 (chorus) |
| pn | 0 - 8 (4) | p0c p2c p4c p6c p8c; |
| @pn | -64 - 64 (0) | @p-64c @p-32c @p0c @p32c @p64c; // same as above |
**The formula for output**
[Monoral output] = ([Argument of 'v'] / 16) x ([Argument of 'x'] / 128)
[Stereo left output]  = [Monoral output] x ([Argument of '@v'] / 128) x cos(([Argument of '@p'] + 64) / 128 x PI/2)
[Stereo Right output] = [Monoral output] x ([Argument of '@v'] / 128) x sin(([Argument of '@p'] + 64) / 128 x PI/2)
The monoral output is used for the bus pipe specifyed by "@o" and the input for "%e", so "@v" and "@p" are ignored in these situation.
### Voice related commands
```mml
%0c %1c %2c %3c;
```
```mml
%5 @0c @1c @2c @3c @4c @5c @6c @7c; // select wave shape by 1st argument
```
```mml
@0,32,28,28,28,2,0 cde;   // modify envelop by 2nd-7th arguments.
```
```mml
%2 @f64,3 q0 c;   // noise with LPF(cutoff=64, resonance=3)
```
```mml
@f0,2,32,32,32 cde;    // filter envelop specifyed by 3rd-10th arguments
```
```mml
%f2 @f96 cde;   // high pass filter
```
```mml
q4 s63cr s32cr s24cr s20cr s12cr;
```
```mml
q4 s28,-32cde;    // pitch sweep after key-off
```
```mml
%5 @al2,0 cde;  // 2ope frequency modulating connection [o1(o0)]
```
```mml
%5 @al2,0 @fb5,1 cde;  // feedback from 2nd operator [o1(o0(o1))]
```
```mml
%5 @al2 i0@0 i1@1 cde;   // The operator#0 outputs sin wave and the operator#1 outputs saw wave.
```
| Statement | Range | Description |
| --- | --- | --- |
| %n1,n2 | n1; 0 - 9 (0) n2; 0 - 7 (0) | %0c %1c %2c %3c; |
| @n1,...n15 | n1; 0 - 1023 (0) | @0,32,28,28,28,2,0 cde; // modify envelop by 2nd-7th arguments. |
| @fn1,...n10 | n1; 0 - 128 (128) n2; 0 - 9 (0) | @f0,2,32,32,32 cde; // filter envelop specifyed by 3rd-10th arguments |
| %fn | n; 0,1,2 (0) | %f2 @f96 cde; // high pass filter |
| sn1,n2 | n1; 0 - 63 (28) n2; -256 - 255 (0) | q4 s28,-32cde; // pitch sweep after key-off |
| @aln1,n2 | n1; 1 - 4 (1) n2; 0 - 15 (0) | %5 @al2,0 cde; // 2ope frequency modulating connection [o1(o0)] |
| @fbn1,n2 | n1; 0 - 7 (0) n2; 0 - 3 (0) | %5 @al2,0 @fb5,1 cde; // feedback from 2nd operator [o1(o0(o1))] |
| in | 0 - 3 (last operators index) | %5 @al2 i0@0 i1@1 cde; // The operator#0 outputs sin wave and the operator#1 outputs saw wave. |
| @rrn1,n2 | n1;0 - 63 (28) n2;-256 - 255 (0) | (@ Release Rate)Same as 5th argument of '@' command.The 2nd argument sets pitch sweep after key-off (same as 2nd argument of 's'). |
| @tln | 0 - 127 (0) | (@ Total Level)Same as 7th argument of '@' command. |
| @mln1,n2 | n1;0 - 15 (1) n2;-128 - 127 (0) | (@ MuLtiple)Same as 10th argument of '@' command.The 2nd argument sets non-integral harmonics. [frequency ratio] = [1st argument(In the case of "0"->0.5)] + [2nd argument] / 128 |
| @dtn | -8192 - 8191 (0) | (@ DeTune)Same as 12th argument of '@' command.'k' modifies all operators, and '@dt' modifies only one operator specifyed by 'i'. |
| @phn | -1 - 255 (0) | (@ PHase)Same as 14th argument of '@' command. |
| @fxn | 0 - 127 (0) | (@ FiXed note)Same as 15th argument of '@' command. |
| @sen | 0 - 17 (0) | (@ SSG Envelpe control)Emulate SSGEC Register of OPNA.The argument of 16 and 17 are extension of SiOPM. 16;Argument=8 with same gain repetition, 17;Argument=12 with same gain repetition. |
| @ern | 0 , 1 | (@ Envelop Reset)Envelop reset.The argument of 1 sets envelop to bottom at the begin of attack. This parameter sets all operators. |
**The 1st argument of '%' and '@'**
Select sound module type by 1st argument of '%', and select tone color index by 1st argument of '@'. The index of tone color depends on selected module type.
(*click index to play sample sound.)
| @0: %0@0cde | Square wave |
| --- | --- |
| @1: #TABLE0{(0,16)16};%0@1nt0,20q8o0c1^1^1 | Pulse Noise(o0c - o2g+ are the NoisePeriod of 0-31 respectively.) |
| @0-7: #TABLE0{(0,8)8};%1@@0,20q8c1. | Duty changable square wave. [Duty]=[argument]x12.5%. '@0' sets output=-1(constant). |
| --- | --- |
| @8: %1@8cde | @8: %1@8cde | NES traiangle wave (8bit quantized). |
| @10: #TABLE0{(0,16)16};%1@10nt0,20q8o0c1^1^1 | 93bit Noise(o0c - o1d+ are NoisePeriod of 0-15 respectively.) |
| @11: | PCM. same as "%7@0"[NOT IMPLEMENTED] |
| @0: %2@0c | White Noise |
| --- | --- |
| @1: %2@1c | Pulse Noise (2 values [1/-1] noise) |
| @2: %2@2c | 93bit Noise (Short noise from 15bit LFSR) |
| @3: %2@3c | Hi-pass Noise (White Noise with high-pass filter) |
| @4: %2@4c | PinkNoise(1/f noise) |
| @8: %2@8cde | Periodic Noise (wave from rotate shifted 16bit LFSR) |
| @9: %2@9cde | 93bit Noise with pitch |
| @10-15: | (reserved) |
| @0: %3@0cde | sin(p) |
| --- | --- |
| @1: %3@1cde | (p<PI)?sin(p):0 |
| @2: %3@2cde | abs(sin(p)) |
| @3: %3@3cde | (p<PI/2\|\|(PI<=p&&p<PI*3/2))?abs(sin(p)):0 |
| @4: %3@4cde | (p<PI)?sin(2p):0 |
| @5: %3@5cde | (p<PI)?abs(sin(2p)):0 |
| @8-13: #TABLE0{(0,6)6}+8;%3@@0,20q8c1 | @0-7 waves modulated by double frequency triangle waves. |
| @16-21: #TABLE0{(0,6)6}+16;%3@@0,20q8c1 | All sins of @0-7 formula are changed into triangle wave function. |
| @24-29: #TABLE0{(0,6)6}+24;%3@@0,20q8c1 | All sins of @0-7 formula are changed into saw wave function. |
| @6: %3@6cde | (p<PI)?1:-1 |
| @14: %3@14cde | (p<PI)?1:0 |
| @22: %3@22cde | (p<PI/2\|\|(PI<=p&&p<PI*3/2))?1:0 |
| @30: %3@30cde | (p<PI/2)?1:0 |
| @7: %3@7cde | (p<PI*0.5)?1-sin(p):(p>PI*1.5)?-1-sin(p):0 |
| @15,23,31 : | Same as %4@[0-2] |
| @0: %5@0cde | Sin wave |
| --- | --- |
| @1: %5@1cde | Upward saw wave |
| @2: %5@2cde | Downward saw wave |
| @3: %5@3cde | NES Triangle wave (8bit Quantized) |
| @4: %5@4cde | Triangle wave |
| @5: %5@5cde | Square wave |
| @6: %5@6cde | White noise (same as @16) |
| @7: %5@7cde | (reserved) ... ? |
| @8: %5@8cde | Pseudo sync for lower freq. output = -p/2PI |
| @9: %5@9cde | Pseudo sync for higher freq. output = p/2PI |
| @10: %5@10cde | Offset. output = 1 |
| @11-15: | (reserved) |
| @16-31: #TABLE0{(16,32)16};%5@@0,20q8c1^1^1 | Noise wave. same as %2@([argument]-16) |
| @32-63: #TABLE0{(32,64)32};%5@@0,10q8c1^1^1 | MA3 wave. same as %3@([argument]-32) |
| @64-95: #TABLE0{(64,96)32};%5@@0,10q8c1^1^1 | Pulse wave. same as %8@([argument]-64) |
| @96-127: | (reserved) |
| @128-255: #TABLE0{(128,256)128};%5@@0,4q8c1^1^1^1 | Ramp wave．same as %9@([argument]-128) |
| @256-511: | Wave table. same as %4@([argument]-256) |
| @0(sample)%7@0l8c.c.gf.f.<c>rccgf.f.<c | * The SWF loaded some mp3 data for demonstration. |
| --- | --- |
| @0-15: #TABLE0{(0,16)16};%8@@0,20q8c1^1^1 | Pulse with 2 states(-1/1). output = (p<PI/8*[index])?1:-1; |
| --- | --- |
| @16-31: #TABLE0{(16,32)16};%8@@0,20q8c1^1^1 |
| @0-63: #TABLE0{(0,64)64};%9@@0,2q8c1^1 | @0=upward-saw -> @64=triangle |
| --- | --- |
| @64-127: #TABLE0{(64,128)64};%9@@0,2q8c1^1 |
| @0(sample)%10v16l8cre4cce.c16rc16c16ercce.e16;%10o4l16[v16c8cv12c]8v16f1 | * The SWF loaded some mp3 data for demonstration. |
| --- | --- |
| @0,48,48,0,0,20s8(default)%11@ph-1o3e1^1^2^8;%11@ph-1o3r8a1^1^2;%11@ph-1o4r4d1^1^4.;%11@ph-1r4.o4g1^1^4;%11@ph-1r2o4b1^1;%11@ph-1r8^2o5e1^1^8; | Classic guitar |
| --- | --- |
| @0,48,48,0,0,16s4#T=%11@0,48,48,0,0,16s4@ph-1q8;t160;To3v12a1^1;To4v8r64e1^1;To4v7r32a1^1;To4v7r32.b1^1;To5v6r16e1^1; | Acoustic guitar |
| @0,48,40,4,0,16s6@f96,0#EFFECT0{chorus delay};#T=%11@0,48,40,4,0,16s6@ph-1@f96,0q8;t160;To3v12$a1^1;To4v8r4$e1^1;To4v7r4.$a4a4a2a4a4a2;To4v7r2.$b2b2b1;To5v6r1$e1^1; | Clean sound (w/ chorus) |
| @0,48,24,8,0,17s2@f80#T=%11@0,48,24,8,0,17s2@f80mp0,6,10,10@ph-1@v64,16;t160;#M=o3q8l8aga<c^2>aga<c*d0rc4.>aga<d*edc>s,-63a1^1;Tv12M;Tv8kt7M; | Distortion |
| @0,48,40,4,80,18s32t144;%11@0,48,40,4,80,18s32q8o2q8l8$v16av12a<c16cc+s40c+16s32deg>a16<s48a16>s32a<c&c+16de16>v8s48ev12s32f+g+; | Slap bass |
**The 2nd-15th argument of '@'**
The '@' command has 15 arguments to modify operators output, 1st argument for tone color, 2nd-7th arguments for envelop, 8th and 9th for key-scaling, 10th-12th for frequency, 13th-15th for others. These parameters are based on OPM. The operators parameter does not changed when the argument abbreviates. For the FM-module emulation (%6), these arguments modify an operator specifyed by 'i'.
```
@ [ws], [ar], [dr], [sr], [rr], [sl], [tl], [ksr], [ksl], [mul], [dt1], [detune], [ams], [phase], [fixed pitch]
```
| n1; | (Wave Shape)Select tone color[0-1023]. See above appendex. |
| --- | --- |
| n2; | (Attack Rate)Output amplification speed after key-on[0-63(63)]. Small values for slower attack. No rising edge (becomes maximum suddenly) for ar=63. Slowest rising for ar=1. |
| n3; | (Decay Rate)Output attenuation after achieving maximum[0-63(0)]. Small values for slower decay. dr=63 makes Sustain Level suddenly after attack. dr=0 keeps maximum output until key-off. |
| n4; | (Sustain Rate)Output attenuation after achieving Sustain Level[0-63(0)]. Small values for slower sustain. sr=63 makes silent suddenly after achieving Sustain Level. sr=0 keeps Sustain Level until key-off. |
| n5; | (Release Rate)Output attenuation after key-off[0-63(28)]. Small values for slower release. rr=63 makes silent suddenly after key-off. rr=0 makes keeps output until next note. same as '@rr'. |
| n6; | (Sustain Level)Output level of changing point from dr to sr. This is relative value with Total Level[0-15(0)]．[Changing point level]=[argument]x-1.5[dB] |
| n7; | (Total Level)Maximum output level of envelop[0-127(0)]. [Maximum level]=[argument]x-0.375[dB]. Same as '@tl'. |
| n8; | (Key Scale Rate)Correction faster envelop for higher pitch[0-3(0)]. 0.4[rate/octave] for ksr=0, 0.8 for ksr=1, 1.6 for ksr=2 and 3.2 for ksr=3. |
| n9; | (Key Scale Level)Correction lower output for higher pitch[0-3(0)]. No correction for ksl=0, -0.15[dB/octave] for ksl=1;，-0.3 for ksl=2 and -0.6 for ksl=3. |
| n10; | (Multiple)Multiplier for wave frequency[0-15(1)]. x0.5 for mul=0. Same as '@ml'. |
| n11; | (Detune1)Parameter 'dt1' of OPM/OPN[0-7(0)]. Fine detune. 0-3 for pitch up and 4-7 for pitch down. |
| n12; | (Detune)Detune [-8192-8191(0)]. The unit is same as 'kt'(64 for 1 halftone). Same as '@dt'. |
| n13; | (Amplitude Modulation Shift)The width of Amplitude Modulation('ma' command) [0-3(1)]. ams=0 sets the amplitude modulation off. |
| n14; | (Phase)Wave phase at the timing of key-on[0-255(0)]. ph=255 makes no phase reset on key-on. ph=-1 randamizes. [phase]=[argument]/128*PI. Same as '@ph'. |
| n15; | (Fixed note)Fix wave frequencey. fx=0 makes not fixed. The argument of 60 means 'o5c'. Same as '@fx'. |
**The arguments for '@al' (or also for the 1st argument of '#@')**
The '@al' command has 2 arguments to modify the number of operators and connecting algorism. The default value of 2nd argument is the parallel connecting argorism (2ope;alg=1, 3ope;alg=5, 4ope;alg=7). *The feedback connection can be modifyed by '@fb', but below figures are written with self-feedback connection in order to simplify.
| Operators | Algolism |
| --- | --- |
| 1 | ![image](./image/opc1alg.png) |
| 2 | ![image](./image/opc2alg.png) |
| 3 | ![image](./image/opc3alg.png) |
| 4 | ![image](./image/opc4alg.png) |
**The table for translate algorisms of various FM sound modules**
The connecting algorism of each FM tone color setting is translated to '@al' (and '@fb') command by below tables.
|  | alg:0 | alg:1 | alg:2 | alg:3 | alg:4 | alg:5 | alg:6 | alg:7 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| 2ope | @al2,0 | @al2,1 | @al2,1 | @al2,1 | @al2,1 | @al2,0 | @al2,1 | @al2,1 |
| 3ope | @al3,0 | @al3,1 | @al3,2 | @al3,3 | @al3,3 | @al3,4 | @al3,3 | @al3,5 |
| 4ope | @al4,0 | @al4,1 | @al4,2 | @al4,3 | @al4,4 | @al4,5 | @al4,6 | @al4,7 |
|  | alg:0 | alg:1 | alg:2 | alg:3 | alg:4 | alg:5 | alg:6 | alg:7 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| 2ope | @al2,0 | @al2,1 | error | error | error | error | error | error |
| 3ope | @al3,0 | @al3,3 | @al3,2 | @al3,2 | error | error | error | error |
| 4ope | @al4,0 | @al4,4 | @al4,8 | @al4,9 | error | error | error | error |
|  | alg:0 | alg:1 | alg:2 | alg:3 | alg:4 | alg:5 | alg:6 | alg:7 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| 2ope | @al2,0 | @al2,1 | @al2,1 | @al2,1 | @al2,0 | @al2,1 | @al2,1 | @al2,1 |
| 3ope | error | error | @al3,5 | @al3,2 | @al3,0 | @al3,3 | @al3,2 | @al3,2 |
| 4ope | error | error | @al4,7 | @al4,2 | @al4,0 | @al4,4 | @al4,8 | @al4,9 |
|  | alg:0 | alg:1 | alg:2 | alg:3 | alg:4 | alg:5 | alg:6 | alg:7 | alg:8 | alg:9 | alg:10 | alg:11 | alg:12 | alg:13 | alg:14 | alg:15 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| 2ope | @al2,0 | @al2,0 @fb0,1 | @al2,1 | @al2,2 | error | error | error | error | error | error | error | error | error | error | error | error |
| 3ope | @al3,0 | @al3,0 @fb0,1 | @al3,1 | @al3,2 | @al3,3 | @al3,3 @fb0,1 | @al3,5 | @al3,6 | error | error | error | error | error | error | error | error |
| 4ope | @al4,0 | @al4,0 @fb0,1 | @al4,1 | @al4,2 | @al4,3 | @al4,3 @fb0,1 | @al4,4 | @al4,4 @fb0,1 | @al4,8 | @al4,11 | @al4,6 | @al4,6 @fb0,1 | @al4,5 | @al4,9 | @al4,12 | @al4,7 |
**The arguments of '@f'**
The '@f' command has 10 arguments, the 1st and 2nd arguments for LPF setting and 3rd-10th arguments for LPF envelop.
```
@f [co], [res], [ar], [dr], [sr], [rr], [co2], [co3], [sc], [rc]
```
| n1; | (Cutoff Freq.)Cut off frequency[0-128(128)]. 128 for full open. |
| --- | --- |
| n2; | (Resonance)Resonance[0-9(0)]. 0 for no resonance, 9 for maximum. |
| n3; | (Attack Rate)Attack[0-63(0)]. The changing speed from Cutoff Freq. to Cutoff Freq.#2. ar=0 keeps Cutoff Freq. ar=63 changes suddenly. |
| n4; | (Decay Rate)Decay[0-63(0)]. The changing speed from Cutoff Freq.#2 to Cutoff Freq.#3．dr=0 keeps Cutoff Freq.#2. dr=63 changes suddenly. |
| n5; | (Sustain Rate)Sustain[0-63(0)]. The changing speed from Cutoff Freq.#3 to Suatain Freq. sr=0 keeps Cutoff Freq.#3. sr=63 changes suddenly. |
| n6; | (Release Rate)Release[0-63(0)]. The changing speed from key-off to Release Freq. rr=0 keeps cutoff frequency at key-off. rr=63 changes suddenly. |
| n7; | (Cutoff Freq.#2)Cutoff frequency #2[0-128(128)]．2nd cutoff frequency after Attack. |
| n8; | (Cutoff Freq.#3)Cutoff frequency #3[0-128(64)]．3rd cutoff frequency after Decay. |
| n9; | (Sustain CutOff)Cutoff frequency after Sustain[0-128(32)]．4th cutoff frequency. This frequency is kept until key-off. |
| n10; | (Release CutOff)Cutoff frequency after key-off[0-128(128)]. The cutoff frequency changes into this value after key-off, when rr>0. |
**%7;PCM and %10;Sampler**
Both of them play mp3 data loaded in the SWF.
PCM sound module can control the pitch and you can use all commands as FM sound module except for @al and @fb.
On the other hand, Sampler cannot change the pitch and you cannot use ADSR envelop and filter, but you can assign waves on each note.
* In this example, the SWF loaded some mp3 data for demonstration.
```mml
// PCM sound module can control pitch.
%7@0l8c.c.gf.f.<c>rccgf.f.<c;
```
```mml
// Sampler assigns a wave on each note.
%10v16l8cre4cce.c16rc16c16ercce.e16;
%10o4l16[v16c8cv12c]8v16f1;
```
**%11; Control the voice of PMS guitar** *(not implemented)*
The voice of physical modeling synthesis guitar (%11) is controled by "@", "s" and "@f" commands.
"@" command defines a plunk profile. The meanings of 2nd and following arguments depend on the 1st argument.
```
@0,[ar],[dr],[tl],[pitch],[ws]
```
When the 1st argument is 0, voice is defined by following 5 arguments.
The 2nd, 3rd and 4th arguments are for the attack rate, decay rate and total level of a plunking energy, respectively. The meanings are same as FM synth.
The 5th argument is for the pitch of plunking noise and specifyed by note number (defalut=60=o5c).
The 6th argument is for the wave shape of plunking noise and specifyed by wave number of %5 (defalut=20=pink noise).
```
@1,[FM voice number]
```
```
@2,[PCM wave number]
```
When the 1st argument is 1 or 2, the 2nd argument specifies FM voice number or PCM wave number, respectively. In this case, you can use FM voice or PCM wave as a plunk noise.
"s" command defines the attenuation of string energy.
"@f" command defines low-pass filter same as other voices.
### Trigger
| Statement | Range | Description |
| --- | --- | --- |
| %tn1,n2,n3 | n1; - (0) n2; 0 - 3 (1) n3; 0 - 3 (1) | (event Trigger)Event trigger setting. This command is used to interlock with ActionScript. The 1st argument sets trigger ID, 2nd argument sets trigger type for note on and 3rd argument sets trigger type for note off. |
| %en1,n2 | n1; - (0) n2; 0 - 3 (1) | (dispatch Event)Dispatch note on event once. This command is used to interlock with ActionScript. The 1st argument sets trigger ID, 2nd argument sets trigger type. |
**Event trigger**
After the "%t" command, SiONTrackEevnt.NOTE_ON_STREAM, SiONTrackEevnt.NOTE_OFF_STREAM, SiONTrackEevnt.NOTE_ON_FRAME and SiONTrackEevnt.NOTE_OFF_FRAME events are dispatched at each timing of note command(c-b).
The 2nd and 3rd argument sets which type of event are dispatching (0;Do nothing, 1;dispatch NOTE_*_FRAME，2;dispatch NOTE_*_STREAM, 3;dispatch both).
The NOTE_*_STREAM event is dispatched when the note appears in the rendering timing, And the NOTE_*_FRAME event is dispatched when the note sounds.
The "%e" command dispatches note on events once at that timing. The arguments are same as "%t".
### Low Frequency Occilator (LFO) Modulation
```mml
@lfo30 mp32c1 @lfo10 mp32c1
```
```mml
mp8,32,60,40 c1^1
```
```mml
ma0,32,60,40 c1^1
```
| Statement | Range | Description |
| --- | --- | --- |
| @lfon1,n2 | n1; 1 - (20) n2; 0 - 3 (2) | @lfo30 mp32c1 @lfo10 mp32c1 |
| man1,n2,n3,n4 | n1; 0 - 8191 (0) n2; 0 - 8192 (0) n3; 0 - 65535 (0) n4; 0 - 65535 (0) | ma0,32,60,40 c1^1 |
**The arguments of 'mp' and 'ma'**
When key is on, modulation is [1st argument]. And if [1st argument] < [2nd argument], it keeps modulation of [1st argument] during [3rd argument] frames, and then it changes modulation into [2nd argument] for [4th argument] frames.
### Table envelope
```mml
#TABLE0{(0,12)30}; q8 nt0 @fps30 c1 @fps120 c1;
```
```mml
#TABLE0{(0,32)32}; q8 %3 @@0,5 c1^1;  // Changes tone color from '%3@0' to '%3@31' for 5 frames each.
```
```mml
#TABLE0{(128,0)10 (64,0)10|(32,0)10}; na0q8 l2cde;
```
```mml
#TABLE0{(0,128,-64,0)60}; np0q8 c1;
```
```mml
#TABLE0{|0,4,7}; nt0 cde; // high speed arpeggio by table
```
```mml
#TABLE0{|(128,64,128)20}; nf0 c1; // Wah-wah
```
```mml
#TABLE0{|1,5};q4 %5 _@@0 cdef;
```
```mml
#TABLE0{32}; _na0 q4s0cder _na255x0c;
```
```mml
#TABLE0{(0,64,0)20}; _np0 q4cde;
```
```mml
#TABLE0{|0,3,7}; _nt0 q4cde;
```
```mml
#TABLE0{(0,128)20}; _nf0 q2cde;
```
| Statement | Range | Description |
| --- | --- | --- |
| @fpsn | 1 - 1000 (default value is specifyed by #FPS) | #TABLE0{(0,12)30}; q8 nt0 @fps30 c1 @fps120 c1; |
| @@n1,n2 | n1; 0 - 255 (255) n2; 1 - 65535 (1) | #TABLE0{(0,32)32}; q8 %3 @@0,5 c1^1; // Changes tone color from '%3@0' to '%3@31' for 5 frames each. |
| nan1,n2 | n1; 0 - 255 (255) n2; 1 - 65535 (1) | #TABLE0{(128,0)10 (64,0)10\|(32,0)10}; na0q8 l2cde; |
| npn1,n2 | n1; 0 - 255 (255) n2; 1 - 65535 (1) | #TABLE0{(0,128,-64,0)60}; np0q8 c1; |
| ntn1,n2 | n1; 0 - 255 (255) n2; 1 - 65535 (1) | #TABLE0{\|0,4,7}; nt0 cde; // high speed arpeggio by table |
| nfn1,n2 | n1; 0 - 255 (255) n2; 1 - 65535 (1) | #TABLE0{\|(128,64,128)20}; nf0 c1; // Wah-wah |
| _@@n1,n2 | n1; 0 - 255 (255) n2; 1 - 65535 (1) | #TABLE0{\|1,5};q4 %5 _@@0 cdef; |
| _nan1,n2 | n1; 0 - 255 (255) n2; 1 - 65535 (1) | #TABLE0{32}; _na0 q4s0cder _na255x0c; |
| _npn1,n2 | n1; 0 - 255 (255) n2; 1 - 65535 (1) | #TABLE0{(0,64,0)20}; _np0 q4cde; |
| _ntn1,n2 | n1; 0 - 255 (255) n2; 1 - 65535 (1) | #TABLE0{\|0,3,7}; _nt0 q4cde; |
| _nfn1,n2 | n1; 0 - 255 (255) n2; 1 - 65535 (1) | #TABLE0{(0,128)20}; _nf0 q2cde; |
| !nan1,n2 | n1; 0 - 255 (255) n2; 1 - 65535 (1) | [NOT RECOMMENDED](Amplitude ENvelope) Relative volume envelop. This command adds table values on original 'x' value. |
**The arguments of table envelop commands**
These table envelop commands refer a table defined by #TABLE. When [1st argument] = 255, the envelop is canceled. And the 2nd argument specifys speed of a envelop by frame count.
```mml
#TABLE0{|0,4,7}; nt0,1c nt0,2c nt0,5c; // 2nd argument sets speed
```
### Bus pipe control commands
```mml
@o1 %5q8s0 cde; @i5 %5 cde;
```
```mml
@o1 %5q8s0 cde; @i3 %5 cde;  // frequency modulation
```
```mml
@o1 %5q8s0 cde; @r5 %5gab;  // ring modulation
```
| Statement | Range | Description |
| --- | --- | --- |
| @on1,n2 | n1; 0 - 2 (0) n2; 0 - 3 (0) | @o1 %5q8s0 cde; @i5 %5 cde; |
| @in1,n2 | n1; 0 - 7 (5) n2; 0 - 3 (0) | @o1 %5q8s0 cde; @i3 %5 cde; // frequency modulation |
| @rn1,n2 | n1; 0 - 8 (4) n2; 0 - 3 (0) | @o1 %5q8s0 cde; @r5 %5gab; // ring modulation |
