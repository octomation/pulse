// Flow mode of pulse: the cards of the page one at a time, with the answers
// always in the same place. Skip and Interesting mark the repository as the
// page does; Later marks nothing. The next card appears only after the
// service said that the mark is on disk.
(function () {
  'use strict';

  var base = document.body.dataset.base || '/';
  var day = document.body.dataset.day || '';
  var latest = document.body.dataset.latest === 'true';
  var cards = Array.prototype.slice.call(document.querySelectorAll('.flow-card[data-repo]'));
  var end = document.getElementById('flow-end');
  var bar = document.getElementById('flow-bar');
  var form = document.getElementById('flow-form');
  var comment = form.querySelector('[name="comment"]');
  var problem = document.getElementById('flow-error');
  var place = document.querySelector('[data-flow="place"]');
  var left = document.querySelector('[data-flow="left"]');
  var reader = document.getElementById('reader');
  var sheet = reader.querySelector('.reader-sheet');
  var readerBody = document.getElementById('reader-body');
  var at = 0;
  var pending = false;
  var asked = 0;

  // The reader shows one card here: it has no neighbours, and the answers
  // stay in the bar.
  Array.prototype.forEach.call(reader.querySelectorAll('[data-action="prev-story"], [data-action="next-story"]'), function (control) {
    control.hidden = true;
  });

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
    say('');
    if (done) {
      var back = end.querySelector('[data-flow="page"]');
      if (back) {
        back.focus({ preventScroll: true });
      }
    }
  }

  // answer writes the answer for the current card and moves on once it is
  // written; a failed write keeps the card on the screen with the error.
  function answer(kind) {
    var card = cards[at];
    if (!card || pending || !reader.hidden) {
      return;
    }
    if (kind === 'later') {
      at++;
      show();
      return;
    }
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
      busy(false);
      at++;
      show();
    }, function (error) {
      busy(false);
      say(error.message);
    });
  }

  function open(card) {
    var name = card.dataset.name;
    var ask = ++asked;
    reader.hidden = false;
    document.body.classList.add('is-reading');
    readerBody.innerHTML = '<p class="reader-wait">Opening…</p>';
    sheet.focus({ preventScroll: true });
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
      if (ask === asked) {
        readerBody.innerHTML = story;
      }
    }, function (error) {
      if (ask !== asked) {
        return;
      }
      readerBody.textContent = '';
      var line = document.createElement('p');
      line.className = 'reader-wait';
      line.textContent = error.message;
      readerBody.appendChild(line);
    });
  }

  function close() {
    asked++;
    reader.hidden = true;
    readerBody.textContent = '';
    document.body.classList.remove('is-reading');
  }

  form.addEventListener('submit', function (event) {
    event.preventDefault();
    answer('interesting');
  });

  document.addEventListener('click', function (event) {
    var control = event.target.closest('[data-answer], [data-action]');
    if (!control || control.disabled) {
      return;
    }
    if (control.dataset.answer) {
      answer(control.dataset.answer);
      return;
    }
    switch (control.dataset.action) {
      case 'read':
        if (event.metaKey || event.ctrlKey || event.shiftKey || event.altKey || event.button !== 0) {
          return;
        }
        if (window.getSelection && String(window.getSelection())) {
          return;
        }
        event.preventDefault();
        open(control.closest('[data-repo]'));
        break;
      case 'close-reader':
        close();
        break;
    }
  });

  document.addEventListener('keydown', function (event) {
    // A held key answers once: an answer cannot be taken back.
    if (event.metaKey || event.ctrlKey || event.altKey || event.shiftKey || event.repeat) {
      return;
    }
    if (!reader.hidden) {
      if (event.key === 'Escape') {
        close();
      }
      return;
    }
    if (event.target.closest('input, textarea')) {
      // In the comment, Enter answers Interesting; Escape leaves the field
      // so that the arrows answer again.
      if (event.key === 'Escape') {
        event.target.blur();
      }
      return;
    }
    var kinds = { ArrowLeft: 'skip', ArrowRight: 'interesting', ArrowDown: 'later' };
    if (kinds[event.key]) {
      event.preventDefault();
      answer(kinds[event.key]);
    }
  });

  // A swipe on the card answers as the arrows do: left skips, right is
  // interesting.
  var start = null;
  var stage = document.getElementById('flow-stage');
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
