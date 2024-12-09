// Flow mode of pulse: the cards of the page one at a time, each with the
// whole story of its repository, and the answers always in the same place.
// Skip and Interesting mark the repository as the page does; Later marks
// nothing; Back returns to the previous card, whose answer another one
// replaces; Take back removes the answer of the card on the screen. The next
// card appears only after the service said that the mark is on disk.
(function () {
  'use strict';

  var base = document.body.dataset.base || '/';
  var day = document.body.dataset.day || '';
  var latest = document.body.dataset.latest === 'true';
  var cards = Array.prototype.slice.call(document.querySelectorAll('.flow-card[data-repo]'));
  var stage = document.getElementById('flow-stage');
  var end = document.getElementById('flow-end');
  var bar = document.getElementById('flow-bar');
  var form = document.getElementById('flow-form');
  var comment = form.querySelector('[name="comment"]');
  var problem = document.getElementById('flow-error');
  var place = document.querySelector('[data-flow="place"]');
  var left = document.querySelector('[data-flow="left"]');
  var backs = Array.prototype.slice.call(document.querySelectorAll('[data-answer="back"]'));
  var at = 0;
  var pending = false;
  // trail holds the places of the cards this flow answered or left for
  // later, in order: Back walks it.
  var trail = [];
  // prefilled says that the comment field took the comment of the card on
  // the screen: it does so once, and never over what was typed.
  var prefilled = false;
  // asked counts the stories asked for each card: an answer that arrives
  // after a newer question, or after the card was answered, is dropped.
  var asked = [];
  // given holds the answer this flow wrote for each card, with its comment:
  // until the story of the card shows it again, the bar goes by it.
  var given = [];

  function post(path, body) {
    return fetch(path, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body)
    }).then(function (response) {
      return response.json().catch(function () { return {}; }).then(function (answer) {
        if (!response.ok) {
          throw new Error(answer.error || 'pulse answered ' + response.status);
        }
        return answer;
      });
    }, function () {
      throw new Error('pulse does not answer: is the service running?');
    });
  }

  function say(text) {
    problem.textContent = text;
    problem.hidden = !text;
  }

  function busy(on) {
    pending = on;
    bar.classList.toggle('is-busy', on);
    Array.prototype.forEach.call(bar.querySelectorAll('button, input'), function (control) {
      control.disabled = on;
    });
    if (!on) {
      sync();
    }
  }

  // load puts the story of the card in its place: the facts, the topics, the
  // links and the README, as the reader shows them. Until it arrives, or
  // when it cannot, the card shows what the lists know.
  function load(i) {
    var card = cards[i];
    if (!card || card.dataset.story) {
      return;
    }
    card.dataset.story = 'loading';
    var ask = asked[i] = (asked[i] || 0) + 1;
    var name = card.dataset.name;
    fetch(base + 'repos/' + name.split('/').map(encodeURIComponent).join('/')).then(function (response) {
      return response.text().then(function (text) {
        if (!response.ok) {
          throw new Error(text.trim() || 'pulse answered ' + response.status);
        }
        return text;
      });
    }, function () {
      throw new Error('pulse does not answer: is the service running?');
    }).then(function (story) {
      if (ask !== asked[i]) {
        return;
      }
      card.innerHTML = story;
      card.dataset.story = 'done';
      if (i === at) {
        sync();
      }
    }, function (error) {
      if (ask !== asked[i]) {
        return;
      }
      delete card.dataset.story;
      // A stamp of before the last answer would say what is no longer so.
      Array.prototype.forEach.call(card.querySelectorAll('[data-stamp]'), function (stamp) { stamp.remove(); });
      var line = card.querySelector('[data-flow="story-error"]') || document.createElement('p');
      line.className = 'story-error';
      line.dataset.flow = 'story-error';
      line.textContent = 'The story could not be opened: ' + error.message;
      card.appendChild(line);
    });
  }

  // fresh reports whether the card shows the story the service gave after
  // its last answer.
  function fresh(i) {
    return cards[i] && cards[i].dataset.story === 'done';
  }

  // standing is the answer of the card at i: interesting, skip, seen,
  // notes, or empty. The story says it once it is fresh; until then, the
  // answer this flow wrote for the card.
  function standing(i) {
    if (fresh(i)) {
      var stamp = cards[i].querySelector('[data-stamp]');
      return stamp ? stamp.dataset.stamp : '';
    }
    var mine = given[i];
    return mine && mine.action !== 'clear' ? mine.action : '';
  }

  // takable reports whether the card at i has an answer of pulse to take
  // back: a card in the notes may have one too.
  function takable(i) {
    if (fresh(i)) {
      return !!cards[i].querySelector('.story [data-action="clear"]');
    }
    return standing(i) !== '';
  }

  // sync shows the answer the current card has on its button, and fills the
  // comment of a bookmark: the one this flow wrote, or the one of its story.
  function sync() {
    var current = standing(at);
    Array.prototype.forEach.call(bar.querySelectorAll('[data-answer]'), function (button) {
      var kind = button.dataset.answer;
      var pressed = kind === 'interesting' ? current === 'interesting' : kind === 'skip' && (current === 'skip' || current === 'seen');
      button.setAttribute('aria-pressed', pressed ? 'true' : 'false');
      if (!pending && (kind === 'skip' || kind === 'interesting')) {
        button.disabled = current === 'notes';
      }
      if (!pending && kind === 'clear') {
        button.disabled = !takable(at);
      }
    });
    backs.forEach(function (button) { button.disabled = pending || trail.length === 0; });
    // A card in the notes takes no Interesting, so no comment either.
    bar.querySelector('[data-action="comment"]').disabled = pending || current === 'notes';
    if (prefilled || comment.value !== '') {
      return;
    }
    var mine = given[at];
    if (mine && mine.action === 'interesting') {
      comment.value = mine.comment;
      prefilled = true;
      return;
    }
    var kept = fresh(at) && cards[at].querySelector('.story [name="comment"]');
    if (kept) {
      comment.value = kept.value;
      prefilled = true;
    }
  }

  // show puts the card at the place on the screen, or the end of the flow
  // after the last one.
  function show() {
    cards.forEach(function (card, i) { card.hidden = i !== at; });
    var done = at >= cards.length;
    end.hidden = !done;
    bar.hidden = done;
    if (place) {
      place.textContent = Math.min(at + 1, cards.length);
      left.textContent = cards.length - at;
    }
    document.title = (done ? '' : '(' + (cards.length - at) + ') ') + 'Flow · pulse';
    comment.value = '';
    prefilled = false;
    closeComment();
    say('');
    window.scrollTo(0, 0);
    if (done) {
      var back = end.querySelector('[data-flow="page"]');
      if (back) {
        back.focus({ preventScroll: true });
      }
    } else {
      load(at);
      load(at + 1);
      stage.focus({ preventScroll: true });
    }
    sync();
  }

  // answer writes the answer for the current card and moves on once it is
  // written; a failed write keeps the card on the screen with the error.
  function answer(kind) {
    if (pending) {
      return;
    }
    if (kind === 'back') {
      back();
      return;
    }
    var card = cards[at];
    if (!card) {
      return;
    }
    if (kind === 'later') {
      trail.push(at);
      at++;
      show();
      return;
    }
    if (kind !== 'clear' && standing(at) === 'notes' || kind === 'clear' && !takable(at)) {
      return;
    }
    var where = at;
    var body = { action: kind, repo: card.dataset.name, comment: kind === 'interesting' ? comment.value : '' };
    if (day) {
      body.day = day;
    }
    if (latest) {
      body.latest = true;
    }
    say('');
    busy(true);
    post('/api/marks', body).then(function (answer) {
      if (answer.reload) {
        window.location.reload();
        return;
      }
      // The story shows the answer the next time it loads; a story still
      // on its way is of before the answer.
      given[where] = { action: body.action, comment: body.comment };
      asked[where] = (asked[where] || 0) + 1;
      delete card.dataset.story;
      if (kind === 'clear') {
        // The card is new again and stays on the screen.
        comment.value = '';
        prefilled = false;
        busy(false);
        load(where);
        return;
      }
      trail.push(where);
      at++;
      busy(false);
      show();
    }, function (error) {
      busy(false);
      say(error.message);
    });
  }

  // back returns to the previous card of the flow, with the answer it got.
  function back() {
    if (!trail.length) {
      return;
    }
    at = trail.pop();
    show();
  }

  function openComment() {
    if (at >= cards.length || pending || standing(at) === 'notes') {
      return;
    }
    form.hidden = false;
    comment.focus();
  }

  function closeComment() {
    form.hidden = true;
    if (document.activeElement === comment) {
      comment.blur();
      stage.focus({ preventScroll: true });
    }
  }

  form.addEventListener('submit', function (event) {
    event.preventDefault();
    answer('interesting');
  });

  document.addEventListener('click', function (event) {
    var control = event.target.closest('[data-answer], [data-action="comment"]');
    if (!control || control.disabled) {
      return;
    }
    if (control.dataset.action === 'comment') {
      openComment();
      return;
    }
    answer(control.dataset.answer);
  });

  document.addEventListener('keydown', function (event) {
    // A held key answers once.
    if (event.metaKey || event.ctrlKey || event.altKey || event.repeat) {
      return;
    }
    if (event.target.closest('input, textarea')) {
      // In the comment, Enter answers Interesting; Escape closes it so
      // that the arrows answer again.
      if (event.key === 'Escape') {
        event.preventDefault();
        closeComment();
      }
      return;
    }
    var kinds = { ArrowLeft: 'skip', ArrowRight: 'interesting', ArrowDown: 'later', ArrowUp: 'back', u: 'clear', U: 'clear' };
    if (event.shiftKey && event.key.indexOf('Arrow') === 0) {
      return;
    }
    if (kinds[event.key]) {
      event.preventDefault();
      answer(kinds[event.key]);
    } else if (event.key === 'c' || event.key === 'C') {
      event.preventDefault();
      openComment();
    }
  });

  // A click into the README gives the keys to its frame, which runs no
  // script of pulse: the flow takes them back, so that the arrows still
  // answer. The README scrolls with the pointer, and its links open.
  window.addEventListener('blur', function () {
    setTimeout(function () {
      var active = document.activeElement;
      if (active && active.tagName === 'IFRAME' && stage.contains(active)) {
        stage.focus({ preventScroll: true });
      }
    }, 0);
  });

  // A swipe on the card answers as the arrows do: left skips, right is
  // interesting. The README keeps its own touches.
  var start = null;
  stage.addEventListener('pointerdown', function (event) {
    if (event.pointerType === 'mouse') {
      return;
    }
    start = { x: event.clientX, y: event.clientY };
  });
  stage.addEventListener('pointercancel', function () { start = null; });
  stage.addEventListener('pointerup', function (event) {
    if (!start) {
      return;
    }
    var dx = event.clientX - start.x;
    var dy = event.clientY - start.y;
    start = null;
    if (Math.abs(dx) < 60 || Math.abs(dx) < 1.5 * Math.abs(dy)) {
      return;
    }
    answer(dx < 0 ? 'skip' : 'interesting');
  });

  show();
})();
